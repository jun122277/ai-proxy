package sdk_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jun122277/ai-proxy/internal/mockllm"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// No API key or external endpoint is used. The transport rejects anything
// except this test's loopback listener, including a redirected request.
type localOnly struct {
	host string
	next http.RoundTripper
}

func (rt localOnly) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" || req.URL.Host != rt.host {
		return nil, fmt.Errorf("SDK contract test attempted a non-local request")
	}
	return rt.next.RoundTrip(req)
}

func TestOfficialSDKContract(t *testing.T) {
	handler, err := mockllm.New(mockllm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	defer srv.Close()
	endpoint, _ := url.Parse(srv.URL)
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := openai.NewClient(
		option.WithBaseURL(srv.URL+"/v1/"),
		option.WithAPIKey("mock-test-key"),
		option.WithMaxRetries(0),
		option.WithHTTPClient(&http.Client{Transport: localOnly{host: endpoint.Host, next: transport}, Timeout: 5 * time.Second}),
	)
	params := openai.ChatCompletionNewParams{
		Model:    mockllm.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Hello")},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Choices) != 1 || result.Choices[0].Message.Content != mockllm.Reply || result.Choices[0].FinishReason != "stop" || result.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected SDK response: %+v", result)
	}
	for _, includeUsage := range []bool{false, true} {
		params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(includeUsage)}
		stream := client.Chat.Completions.NewStreaming(ctx, params)
		var content string
		var chunks, finishes, usageEvents int
		for stream.Next() {
			chunk := stream.Current()
			chunks++
			if len(chunk.Choices) == 0 {
				usageEvents++
				if chunk.Usage.TotalTokens != 5 {
					t.Fatal("SDK did not decode usage")
				}
				continue
			}
			content += chunk.Choices[0].Delta.Content
			if chunk.Choices[0].FinishReason == "stop" {
				finishes++
			}
		}
		err := stream.Err()
		_ = stream.Close()
		wantChunks, wantUsage := 6, 0
		if includeUsage {
			wantChunks, wantUsage = 7, 1
		}
		if err != nil || content != mockllm.Reply || chunks != wantChunks || finishes != 1 || usageEvents != wantUsage {
			t.Fatalf("SSE: content=%q chunks=%d finishes=%d usage=%d error=%v", content, chunks, finishes, usageEvents, err)
		}
	}
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{}
	params.MaxCompletionTokens = openai.Int(2)
	result, err = client.Chat.Completions.New(ctx, params)
	if err != nil || result.Choices[0].Message.Content != "Hello from" || result.Choices[0].FinishReason != "length" {
		t.Fatalf("output cap contract: response=%+v error=%v", result, err)
	}
	params.Model = "unsupported-model"
	_, err = client.Chat.Completions.New(ctx, params)
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected an SDK API error, got %v", err)
	}
}
