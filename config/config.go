// Package config provides types and functions for loading governance configuration.
//
// Configuration is loaded from YAML files, with support for per-repo overrides
// that allow certain rules to be customized for specific repositories.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level governance configuration loaded from YAML.
// It defines the organization, GitHub App credentials, and which rules are enabled.
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

// RuleConfig represents the configuration for a single governance rule.
// Scope specifies whether the rule operates at the repository or organization level.
// Settings contains rule-specific configuration parsed by each rule.
type RuleConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Scope    string         `yaml:"scope"` // "repo" or "org"
	Settings map[string]any `yaml:"settings"`
}

// RepoOverride represents per-repo override configuration loaded from overrides/<repo>.yaml.
// It allows certain rules to be customized for specific repositories on top of global rules.
type RepoOverride struct {
	Rules map[string]RepoOverrideRule `yaml:"rules"`
}

// RepoOverrideRule holds override settings for a single rule within a repo override file.
// The settings structure must match what the corresponding rule expects.
type RepoOverrideRule struct {
	Settings map[string]any `yaml:"settings"`
}

// Load reads and parses the governance configuration from the given YAML file.
// Returns an error if the file cannot be read or contains invalid YAML.
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

// LoadOverrides reads all YAML override files from the given directory.
// It returns a map mapping repository names to their override configuration.
// Repository names are derived from filenames (e.g., "payment-service.yaml" → "payment-service").
// Returns an empty map if the directory does not exist.
func LoadOverrides(dir string) (map[string]RepoOverride, error) {
	overrides := make(map[string]RepoOverride)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return overrides, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checking overrides directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("overrides path %q is not a directory", dir)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("globbing overrides directory: %w", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading override file %s: %w", file, err)
		}

		var override RepoOverride
		if err := yaml.Unmarshal(data, &override); err != nil {
			return nil, fmt.Errorf("parsing override file %s: %w", file, err)
		}

		repoName := strings.TrimSuffix(filepath.Base(file), ".yaml")
		overrides[repoName] = override
	}

	return overrides, nil
}
