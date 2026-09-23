package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jun122277/ai-proxy/internal/config"
)

func TestHealthAndUnimplementedRoutes(t *testing.T) {
	cfg, _ := config.Load("")
	srv := httptest.NewServer(New(cfg).Handler)
	defer srv.Close()
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
		{http.MethodPost, "/v1/chat/completions", http.StatusNotFound},
		{http.MethodGet, "/readyz", http.StatusNotFound},
	} {
		req, err := http.NewRequest(tc.method, srv.URL+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != tc.status {
			t.Fatalf("%s %s: status=%d, error=%v", tc.method, tc.path, resp.StatusCode, err)
		}
		if tc.status == http.StatusOK && string(body) != "{\"status\":\"ok\"}\n" {
			t.Fatalf("unexpected health response: %q", body)
		}
	}
}

func TestShutdown(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "drain"
		if force {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			exited := make(chan struct{})
			srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(exited)
				close(entered)
				select {
				case <-release:
					_, _ = w.Write([]byte("completed"))
				case <-r.Context().Done():
				}
			})}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = srv.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stopped := make(chan error, 1)
			grace := 2 * time.Second
			if force {
				grace = 30 * time.Millisecond
			}
			go func() { stopped <- Serve(ctx, srv, listener, grace) }()
			response := make(chan string, 1)
			go func() {
				client := &http.Client{Timeout: 3 * time.Second}
				resp, err := client.Get("http://" + listener.Addr().String())
				if err != nil {
					response <- "request failed"
					return
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				response <- string(body)
			}()
			wait(t, entered)
			cancel()
			if !force {
				// An in-flight request must keep shutdown pending.
				select {
				case err := <-stopped:
					t.Fatalf("shutdown returned before handler completion: %v", err)
				case <-time.After(30 * time.Millisecond):
				}
				close(release)
			}
			select {
			case err := <-stopped:
				if force && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected shutdown deadline, got %v", err)
				}
				if !force && err != nil {
					t.Fatal(err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("shutdown did not return")
			}
			wait(t, exited)
			if body := <-response; !force && body != "completed" {
				t.Fatalf("in-flight response lost: %q", body)
			}
		})
	}
}

func wait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(4 * time.Second):
		t.Fatal("handler did not reach expected state")
	}
}
