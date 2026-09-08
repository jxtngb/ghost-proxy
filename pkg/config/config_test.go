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

    if cfg.ServerAddress != "127.0.0.1:443" {
        t.Errorf("unexpected server address: %s", cfg.ServerAddress)
    }

    if cfg.LogLevel != "info" {
        t.Errorf("unexpected log level: %s", cfg.LogLevel)
    }
}
