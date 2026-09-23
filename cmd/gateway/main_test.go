package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckConfigAndFlags(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if err := run([]string{"-check-config"}, logger); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-unknown=do-not-log-this"}, {"unexpected"},
		{"-check-config", "-healthcheck"}, {"-config", "missing.local.json"},
	} {
		err := run(args, logger)
		if err == nil || strings.Contains(err.Error(), "do-not-log-this") {
			t.Fatalf("expected sanitized flag/config error, got %v", err)
		}
	}
}

func TestProbe(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusFound} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/healthz" {
				t.Errorf("unexpected probe path %q", r.URL.Path)
			}
			w.Header().Set("Location", "/healthz")
			w.WriteHeader(status)
		}))
		err := probe(strings.TrimPrefix(srv.URL, "http://"))
		srv.Close()
		if (err == nil) != (status == http.StatusOK) {
			t.Fatalf("HTTP %d: unexpected probe result %v", status, err)
		}
	}
}
