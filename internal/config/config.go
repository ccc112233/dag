package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is process configuration.
type Config struct {
	Listen   string        `yaml:"listen"`
	APIToken string        `yaml:"apiToken"`
	DBPath   string        `yaml:"dbPath"`
	Buffer   BufferConfig  `yaml:"buffer"`
	TSCache  TSCacheConfig `yaml:"tsCache"`
}

// BufferConfig controls outbound spill buffer.
type BufferConfig struct {
	MaxItems int `yaml:"maxItems"`
}

// TSCacheConfig controls the lightweight local time-series sample cache.
type TSCacheConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Path          string `yaml:"path"`
	RetentionDays int    `yaml:"retentionDays"`
}

// Default returns defaults.
func Default() Config {
	return Config{
		Listen:   ":8080",
		APIToken: "dev-token",
		DBPath:   "dap.db",
		Buffer:   BufferConfig{MaxItems: 10000},
		TSCache: TSCacheConfig{
			Enabled:       true,
			Path:          "data/ts",
			RetentionDays: 7,
		},
	}
}

// LoadYAML loads config from a YAML file, overlaying defaults.
func LoadYAML(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "dap.db"
	}
	if cfg.Buffer.MaxItems <= 0 {
		cfg.Buffer.MaxItems = 10000
	}
	if cfg.TSCache.Path == "" {
		cfg.TSCache.Path = "data/ts"
	}
	if cfg.TSCache.RetentionDays <= 0 {
		cfg.TSCache.RetentionDays = 7
	}
	// enabled defaults to true when key omitted: treat zero-value as enabled
	// unless explicitly set false via YAML `enabled: false`.
	if tok := os.Getenv("DAP_API_TOKEN"); tok != "" {
		cfg.APIToken = tok
	}
	return cfg, nil
}
