package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level governance configuration loaded from YAML.
type Config struct {
	GitHubApp   GitHubAppConfig       `yaml:"github-app"`
	Org         string                `yaml:"org"`
	Concurrency int                   `yaml:"concurrency"`
	Rules       map[string]RuleConfig `yaml:"rules"`
}

// GitHubAppConfig holds credentials for authenticating as a GitHub App.
type GitHubAppConfig struct {
	AppID          int64  `yaml:"app-id"`
	InstallationID int64  `yaml:"installation-id"`
	PrivateKeyPath string `yaml:"private-key-path"`
}

// RuleConfig represents a single rule's toggle and settings from the config file.
type RuleConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Settings map[string]any `yaml:"settings"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
