package config

import "testing"

func TestLoad(t *testing.T) {
	cfg, err := Load("../../configs/client.yaml")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.ListenAddress != "127.0.0.1:1080" {
		t.Errorf("unexpected listen address: %s", cfg.ListenAddress)
	}

	if cfg.ServerAddress != "ghostproxy.duckdns.org:443" {
		t.Errorf("unexpected server address: %s", cfg.ServerAddress)
	}

	if cfg.ServerName != "ghostproxy.duckdns.org" {
		t.Errorf("unexpected server name: %s", cfg.ServerName)
	}

	if cfg.CAFile != "" {
		t.Errorf("unexpected CA file: %s", cfg.CAFile)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("unexpected log level: %s", cfg.LogLevel)
	}

	if cfg.FallbackAddress != "" {
		t.Errorf("unexpected fallback address: %s", cfg.FallbackAddress)
	}

	if !cfg.PaddingEnabled {
		t.Error("padding should be enabled")
	}

	if cfg.JitterMS != 0 {
		t.Errorf("unexpected jitter: %d", cfg.JitterMS)
	}
}
