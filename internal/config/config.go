// Package config loads the non-secret startup configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

const maxConfigBytes = 64 << 10

// Config contains only settings that are needed by the M0 HTTP server.
// Credentials will be supplied separately when upstream access is implemented.
type Config struct {
	ListenAddress     string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type fileConfig struct {
	ListenAddress     string `json:"listen_address"`
	ReadHeaderTimeout string `json:"read_header_timeout"`
	IdleTimeout       string `json:"idle_timeout"`
	ShutdownTimeout   string `json:"shutdown_timeout"`
}

// Load uses safe local defaults when path is empty. Unknown fields, extra JSON
// documents and oversized files are rejected; parser errors never echo values.
func Load(path string) (Config, error) {
	raw := fileConfig{
		ListenAddress: "127.0.0.1:8080", ReadHeaderTimeout: "5s",
		IdleTimeout: "60s", ShutdownTimeout: "10s",
	}
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return Config{}, errors.New("cannot open configuration file")
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
		if err != nil || len(data) > maxConfigBytes {
			return Config{}, errors.New("configuration is unreadable or exceeds 64 KiB")
		}
		if err := decode(data, &raw); err != nil {
			return Config{}, err
		}
	}
	return validate(raw)
}

func validate(raw fileConfig) (Config, error) {
	host, port, err := net.SplitHostPort(raw.ListenAddress)
	if err != nil || (host != "localhost" && net.ParseIP(host) == nil) {
		return Config{}, errors.New("listen_address must contain an IP address or localhost and a port")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return Config{}, errors.New("listen_address port must be between 1 and 65535")
	}
	cfg := Config{ListenAddress: raw.ListenAddress}
	for _, item := range []struct {
		name  string
		value string
		dest  *time.Duration
	}{
		{"read_header_timeout", raw.ReadHeaderTimeout, &cfg.ReadHeaderTimeout},
		{"idle_timeout", raw.IdleTimeout, &cfg.IdleTimeout},
		{"shutdown_timeout", raw.ShutdownTimeout, &cfg.ShutdownTimeout},
	} {
		d, err := time.ParseDuration(item.value)
		if err != nil || d <= 0 || d > time.Hour {
			return Config{}, fmt.Errorf("%s must be a duration greater than zero and at most 1h", item.name)
		}
		*item.dest = d
	}
	return cfg, nil
}
