package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitHubApp   GitHubAppConfig       `yaml:"github_app"`
	Org         string                `yaml:"org"`
	Mode        string                `yaml:"mode"`
	Concurrency int                   `yaml:"concurrency"`
	TargetRepo  string                `yaml:"-"` // set via CLI flag only, not config file
	Output      OutputConfig          `yaml:"output"`
	Rules       map[string]RuleConfig `yaml:"rules"`
}

type OutputConfig struct {
	Console bool   `yaml:"console"`
	JSON    string `yaml:"json"` // file path; empty = disabled
}

type GitHubAppConfig struct {
	AppID          int64  `yaml:"app_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKeyPath string `yaml:"private_key_path"`
}

type RuleConfig struct {
	Enabled  bool                   `yaml:"enabled"`
	Settings map[string]interface{} `yaml:"settings"`
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
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Mode == "" {
		cfg.Mode = "evaluate"
	}
	if cfg.Mode != "evaluate" && cfg.Mode != "apply" {
		return nil, fmt.Errorf("invalid mode %q: must be \"evaluate\" or \"apply\"", cfg.Mode)
	}
	if !cfg.Output.Console && cfg.Output.JSON == "" {
		cfg.Output.Console = true
	}
	return &cfg, nil
}
