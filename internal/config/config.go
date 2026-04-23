package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamwork")
}

func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

func Load() (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(Path())
	if err == nil {
		_ = yaml.Unmarshal(data, cfg)
	}

	if v := os.Getenv("TEAMWORK_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("TEAMWORK_TOKEN"); v != "" {
		cfg.Token = v
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0600)
}

func Set(key, value string) error {
	cfg, _ := Load()
	switch key {
	case "url":
		cfg.URL = value
	case "token":
		cfg.Token = value
	default:
		return fmt.Errorf("unknown config key: %s (valid keys: url, token)", key)
	}
	return Save(cfg)
}

func Get(key string) (string, error) {
	cfg, _ := Load()
	switch key {
	case "url":
		return cfg.URL, nil
	case "token":
		return cfg.Token, nil
	default:
		return "", fmt.Errorf("unknown config key: %s (valid keys: url, token)", key)
	}
}
