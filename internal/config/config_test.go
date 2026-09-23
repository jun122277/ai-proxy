package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Run("local defaults", func(t *testing.T) {
		cfg, err := Load("")
		if err != nil || cfg.ListenAddress != "127.0.0.1:8080" || cfg.ShutdownTimeout != 10*time.Second {
			t.Fatalf("unexpected defaults: %+v, %v", cfg, err)
		}
	})
	t.Run("partial override", func(t *testing.T) {
		cfg, err := Load(writeConfig(t, `{"listen_address":"[::1]:9090","shutdown_timeout":"2s"}`))
		if err != nil || cfg.ListenAddress != "[::1]:9090" || cfg.ShutdownTimeout != 2*time.Second || cfg.IdleTimeout != time.Minute {
			t.Fatalf("unexpected override: %+v, %v", cfg, err)
		}
	})
	for name, input := range map[string]string{
		"unknown field":    `{"credential":"do-not-log-this"}`,
		"invalid type":     `{"idle_timeout":42}`,
		"negative timeout": `{"shutdown_timeout":"-1s"}`,
		"zero timeout":     `{"read_header_timeout":"0s"}`,
		"huge timeout":     `{"idle_timeout":"2h"}`,
		"invalid duration": `{"idle_timeout":"do-not-log-this"}`,
		"empty bind":       `{"listen_address":":8080"}`,
		"invalid port":     `{"listen_address":"127.0.0.1:65536"}`,
		"ephemeral port":   `{"listen_address":"127.0.0.1:0"}`,
		"duplicate":        `{"idle_timeout":"1s","idle_timeout":"2s"}`,
		"null field":       `{"idle_timeout":null}`,
		"null document":    `null`,
		"array":            `[]`,
		"empty":            ``,
		"trailing JSON":    `{} {}`,
		"trailing junk":    `{} do-not-log-this`,
		"malformed":        `{"idle_timeout":`,
		"oversized":        strings.Repeat(" ", maxConfigBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(writeConfig(t, input))
			if err == nil {
				t.Fatal("invalid configuration was accepted")
			}
			if strings.Contains(err.Error(), "do-not-log-this") {
				t.Fatal("configuration value leaked through error")
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
			t.Fatal("missing explicit configuration must not fall back to defaults")
		}
	})
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
