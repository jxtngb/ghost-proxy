package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddress       string   `yaml:"listen_address"`
	ServerAddress       string   `yaml:"server_address"`
	LogLevel            string   `yaml:"log_level"`
	CertFile            string   `yaml:"cert_file"`
	KeyFile             string   `yaml:"key_file"`
	FallbackAddress     string   `yaml:"fallback_address"`
	ServerName          string   `yaml:"server_name"`
	CAFile              string   `yaml:"ca_file"`
	AllowedDestinations []string `yaml:"allowed_destinations"`
	PaddingEnabled      *bool    `yaml:"padding_enabled"`
	JitterMS            int      `yaml:"jitter_ms"`
	AllowRemoteBind     bool     `yaml:"allow_remote_bind"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.JitterMS < 0 || cfg.JitterMS > 5000 {
		return Config{}, fmt.Errorf("parse config: jitter_ms must be between 0 and 5000")
	}

	return cfg, nil
}
