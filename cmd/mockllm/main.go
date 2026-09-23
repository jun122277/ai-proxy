// mockllm is a local test fixture. It never contacts a real provider.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jun122277/ai-proxy/internal/mockllm"
	"github.com/jun122277/ai-proxy/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("mockllm stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	flags := flag.NewFlagSet("mockllm", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	address := flags.String("listen", "127.0.0.1:9090", "mock listen address")
	health := flags.Bool("healthcheck", false, "probe the mock listener and exit")
	first := flags.Duration("first-event-delay", 0, "delay before response headers/first SSE event")
	interval := flags.Duration("chunk-interval", 0, "interval before each text fragment")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return fmt.Errorf("usage: mockllm [-listen host:port] [-first-event-delay duration] [-chunk-interval duration]")
	}
	handler, err := mockllm.New(mockllm.Options{FirstEventDelay: *first, ChunkInterval: *interval})
	if err != nil {
		return err
	}
	if *health {
		return probe(*address)
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		return fmt.Errorf("cannot bind mock listener")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	logger.Info("mockllm listening", "address", listener.Addr().String())
	// Reuse the M0 process lifecycle for the fixture; gateway streaming
	// timeout/drain decisions are intentionally left to M1-02.
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute}
	return server.Serve(ctx, srv, listener, 5*time.Second)
}

func probe(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid mock listen address")
	}
	if host == "0.0.0.0" || host == "" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return fmt.Errorf("mock healthcheck failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mock healthcheck returned HTTP %d", resp.StatusCode)
	}
	return nil
}
