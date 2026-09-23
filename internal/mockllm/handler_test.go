package mockllm

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWireContract(t *testing.T) {
	handler, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, streaming := range []bool{false, true} {
		body := `{"model":"mock-model","messages":[{"role":"user","content":"private-prompt"}]}`
		if streaming {
			body = strings.TrimSuffix(body, "}") + `,"stream":true,"stream_options":{"include_usage":true}}`
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "private-prompt") {
			t.Fatalf("unexpected response: HTTP %d %s", rec.Code, rec.Body.String())
		}
		if !streaming {
			var result response
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Object != "chat.completion" || result.Choices[0].Message.Content != Reply || result.Usage.TotalTokens != 5 {
				t.Fatalf("unexpected normal response: %+v", result)
			}
			continue
		}
		if rec.Header().Get("Content-Type") != "text/event-stream" || !rec.Flushed {
			t.Fatal("stream must use SSE and flush events")
		}
		frames := strings.Split(strings.TrimSuffix(rec.Body.String(), "\n\n"), "\n\n")
		if len(frames) != 8 || frames[7] != "data: [DONE]" {
			t.Fatalf("expected role, four fragments, finish, usage, DONE: %q", frames)
		}
		var id, text string
		for i, frame := range frames[:7] {
			var event response
			if err := json.Unmarshal([]byte(strings.TrimPrefix(frame, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				id = event.ID
				if event.Choices[0].Delta.Role != "assistant" {
					t.Fatal("first event must identify assistant")
				}
			}
			if event.ID != id || event.Object != "chat.completion.chunk" {
				t.Fatal("stream metadata changed")
			}
			if i < 6 {
				text += event.Choices[0].Delta.Content
				if event.Usage != nil {
					t.Fatal("usage arrived before final usage event")
				}
			}
			if i == 5 && (event.Choices[0].FinishReason == nil || *event.Choices[0].FinishReason != "stop") {
				t.Fatal("missing finish reason")
			}
			if i == 6 && (len(event.Choices) != 0 || event.Usage == nil || event.Usage.TotalTokens != 5) {
				t.Fatal("invalid usage event")
			}
		}
		if text != Reply {
			t.Fatalf("unexpected assembled reply: %q", text)
		}
	}
}

func TestInvalidRequests(t *testing.T) {
	handler, _ := New(Options{})
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{`, 400}, {`{} {}`, 400}, {`null`, 400},
		{`{"model":"wrong","messages":[{"role":"user","content":"test"}]}`, 400},
		{`{"model":"mock-model","messages":[]}`, 400},
		{`{"model":"mock-model","messages":[{"role":"tool","content":"test"}]}`, 400},
		{`{"model":"mock-model","messages":[{"role":"user","content":[]}]}`, 400},
		{`{"model":"mock-model","messages":[{"role":"user"}]}`, 400},
		{`{"model":"mock-model","messages":[{"role":"user","content":"test"}],"tools":[]}`, 400},
		{`{"model":"mock-model","messages":[{"role":"user","content":"test"}],"stream_options":{}}`, 400},
		{`{"model":"mock-model","messages":[{"role":"user","content":"test"}],"max_completion_tokens":0}`, 400},
		{`{"do-not-echo-this":"secret"}`, 400},
		{`{"model":"mock-model","messages":[{"role":"user","content":"` + strings.Repeat("x", maxBodyBytes) + `"}]}`, 413},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want || !strings.Contains(rec.Body.String(), `"type":"invalid_request_error"`) || strings.Contains(rec.Body.String(), "do-not-echo-this") {
			t.Fatalf("unexpected invalid request response: HTTP %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestFirstEventArrivesBeforeCompletion(t *testing.T) {
	handler, _ := New(Options{ChunkInterval: time.Minute})
	finished := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(srv.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"mock-model","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"role":"assistant"`) {
		t.Fatal("missing first streamed event")
	}
	select {
	case <-finished:
		t.Fatal("the response was buffered until completion")
	default:
	}
	// Closing now must let the fixture leave its timed waits; this tests only
	// the mock, not cancellation propagation through the future gateway.
	_ = resp.Body.Close()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("the fixture did not leave its timed wait after client disconnect")
	}
}

func TestOptions(t *testing.T) {
	for _, opts := range []Options{{FirstEventDelay: -1}, {ChunkInterval: 2 * time.Minute}} {
		if _, err := New(opts); err == nil {
			t.Fatal("invalid mock delays were accepted")
		}
	}
}
