package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultAPIURL = "https://api.cubepath.com"
	configDir     = ".cubecli"
	configFile    = "config.json"
)

type Config struct {
	APIToken string `json:"api_token"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir)
}

func Path() string {
	return filepath.Join(Dir(), configFile)
}

func Load() (*Config, error) {
	if token := os.Getenv("CUBE_API_TOKEN"); token != "" {
		return &Config{APIToken: token}, nil
	}

	data, err := os.ReadFile(Path())
	if err != nil {
		return nil, fmt.Errorf("no API token found: set CUBE_API_TOKEN or run 'cubecli config setup'")
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}

	if cfg.APIToken == "" {
		return nil, fmt.Errorf("no API token found: run 'cubecli config setup'")
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(Path(), data, 0600)
}

func APIURL() string {
	if url := os.Getenv("CUBE_API_URL"); url != "" {
		return url
	}
	return DefaultAPIURL
}
