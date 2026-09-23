// Package mockllm implements a deterministic, local-only upstream fixture.
// Its timing knobs are test inputs, not gateway timeout policy.
package mockllm

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	Model        = "mock-model"
	Reply        = "Hello from mock."
	maxBodyBytes = 64 << 10
)

var fragments = []string{"Hello", " from", " mock", "."}

type Options struct {
	FirstEventDelay time.Duration
	ChunkInterval   time.Duration
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string  `json:"role"`
		Content *string `json:"content"`
	} `json:"messages"`
	Stream              bool `json:"stream"`
	MaxCompletionTokens *int `json:"max_completion_tokens"`
	StreamOptions       *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type choice struct {
	Index        int      `json:"index"`
	Message      *message `json:"message,omitempty"`
	Delta        *delta   `json:"delta,omitempty"`
	FinishReason *string  `json:"finish_reason"`
}

type response struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []choice `json:"choices"`
	Usage   *usage   `json:"usage"`
}

func New(opts Options) (http.Handler, error) {
	if opts.FirstEventDelay < 0 || opts.FirstEventDelay > time.Minute || opts.ChunkInterval < 0 || opts.ChunkInterval > time.Minute {
		return nil, errors.New("mock delays must be between 0s and 1m")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"status\":\"ok\"}\n")
	})
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req request
		if err := dec.Decode(&req); err != nil {
			writeDecodeError(w, err)
			return
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			writeDecodeError(w, err)
			return
		}
		if err := validate(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		count, reason := len(fragments), "stop"
		if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens < count {
			count, reason = *req.MaxCompletionTokens, "length"
		}
		result := response{
			ID: "chatcmpl-mock-" + rand.Text(), Object: "chat.completion",
			Created: time.Now().Unix(), Model: Model,
			Choices: []choice{{Index: 0, Message: &message{Role: "assistant", Content: strings.Join(fragments[:count], "")}, FinishReason: &reason}},
			// Synthetic usage: one input unit per message, one output unit per
			// fixture fragment. This is deliberately not a real tokenizer.
			Usage: &usage{PromptTokens: len(req.Messages), CompletionTokens: count, TotalTokens: len(req.Messages) + count},
		}
		if !wait(r.Context(), opts.FirstEventDelay) {
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if !req.Stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		stream(w, r, opts, req, result, count, reason)
	})
	return mux, nil
}

func validate(req request) error {
	if req.Model != Model {
		return errors.New("model must be mock-model")
	}
	if len(req.Messages) == 0 {
		return errors.New("messages must contain at least one text message")
	}
	for _, msg := range req.Messages {
		if msg.Content == nil {
			return errors.New("message content must be a string")
		}
		switch msg.Role {
		case "system", "developer", "user", "assistant":
		default:
			return errors.New("unsupported message role")
		}
	}
	if req.MaxCompletionTokens != nil && (*req.MaxCompletionTokens < 1 || *req.MaxCompletionTokens > 4096) {
		return errors.New("max_completion_tokens must be between 1 and 4096")
	}
	if req.StreamOptions != nil && !req.Stream {
		return errors.New("stream_options requires stream=true")
	}
	return nil
}

func stream(w http.ResponseWriter, r *http.Request, opts Options, req request, result response, count int, reason string) {
	w.Header().Set("Content-Type", "text/event-stream")
	result.Object = "chat.completion.chunk"
	u := result.Usage
	result.Usage = nil
	result.Choices = []choice{{Index: 0, Delta: &delta{Role: "assistant"}}}
	if !event(w, result) {
		return
	}
	for _, part := range fragments[:count] {
		if !wait(r.Context(), opts.ChunkInterval) {
			return
		}
		result.Choices = []choice{{Index: 0, Delta: &delta{Content: part}}}
		if !event(w, result) {
			return
		}
	}
	result.Choices = []choice{{Index: 0, Delta: &delta{}, FinishReason: &reason}}
	if !event(w, result) {
		return
	}
	if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
		result.Choices, result.Usage = []choice{}, u
		if !event(w, result) {
			return
		}
	}
	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err == nil {
		_ = http.NewResponseController(w).Flush()
	}
}

func event(w http.ResponseWriter, data response) bool {
	b, err := json.Marshal(data)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return false
	}
	return http.NewResponseController(w).Flush() == nil
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 64 KiB")
		return
	}
	writeError(w, http.StatusBadRequest, "expected one JSON object with supported fields and types")
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": "invalid_request_error", "param": nil, "code": nil},
	})
}
