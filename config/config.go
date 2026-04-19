package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// RuleConfig represents a single rule's toggle, scope, and settings from the config file.
type RuleConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Scope    string         `yaml:"scope"` // "repo" or "org"
	Settings map[string]any `yaml:"settings"`
}

// RepoOverride represents the per-repo override configuration loaded from overrides/<repo>.yaml.
type RepoOverride struct {
	Rules map[string]RepoOverrideRule `yaml:"rules"`
}

// RepoOverrideRule holds the override settings for a single rule within a repo override file.
type RepoOverrideRule struct {
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

// LoadOverrides reads all YAML files from the given directory and returns a map
// of repo name to its override configuration. The repo name is derived from the
// filename (e.g., "overrides/payment-service.yaml" → "payment-service").
// Returns an empty map (not an error) if the directory does not exist.
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
