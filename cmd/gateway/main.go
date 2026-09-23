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

	"github.com/jun122277/ai-proxy/internal/config"
	"github.com/jun122277/ai-proxy/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	flags := flag.NewFlagSet("gateway", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("config", "", "path to non-secret JSON configuration")
	check := flags.Bool("check-config", false, "validate configuration and exit")
	health := flags.Bool("healthcheck", false, "probe the configured local HTTP listener")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return fmt.Errorf("usage: gateway [-config path] [-check-config | -healthcheck]")
	}
	if *check && *health {
		return fmt.Errorf("check-config and healthcheck are mutually exclusive")
	}
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	if *check {
		logger.Info("configuration valid")
		return nil
	}
	if *health {
		return probe(cfg.ListenAddress)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	logger.Info("gateway listening", "address", listener.Addr().String())
	if err := server.Serve(ctx, server.New(cfg), listener, cfg.ShutdownTimeout); err != nil {
		return err
	}
	logger.Info("gateway shutdown complete")
	return nil
}

func probe(address string) error {
	host, port, _ := net.SplitHostPort(address)
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	client := &http.Client{
		Timeout:       2 * time.Second,
		Transport:     &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return fmt.Errorf("healthcheck failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", resp.StatusCode)
	}
	return nil
}
