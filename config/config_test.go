package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg, err := Load("testdata/valid.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Org != "test-org" {
			t.Errorf("org = %q, want %q", cfg.Org, "test-org")
		}
		if cfg.Concurrency != 5 {
			t.Errorf("concurrency = %d, want %d", cfg.Concurrency, 5)
		}
		if cfg.GitHubApp.AppID != 12345 {
			t.Errorf("app-id = %d, want %d", cfg.GitHubApp.AppID, 12345)
		}
		if cfg.GitHubApp.InstallationID != 67890 {
			t.Errorf("installation-id = %d, want %d", cfg.GitHubApp.InstallationID, 67890)
		}
		if cfg.GitHubApp.PrivateKeyPath != "./key.pem" {
			t.Errorf("private-key-path = %q, want %q", cfg.GitHubApp.PrivateKeyPath, "./key.pem")
		}

		if len(cfg.Rules) != 2 {
			t.Fatalf("rules count = %d, want %d", len(cfg.Rules), 2)
		}

		rulesets := cfg.Rules["repo-rulesets"]
		if !rulesets.Enabled {
			t.Error("repo-rulesets should be enabled")
		}
		if rulesets.Scope != "repo" {
			t.Errorf("repo-rulesets scope = %q, want %q", rulesets.Scope, "repo")
		}
		if rulesets.Settings == nil {
			t.Fatal("repo-rulesets settings should not be nil")
		}

		codeowners := cfg.Rules["codeowners"]
		if codeowners.Enabled {
			t.Error("codeowners should be disabled")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := Load("testdata/nonexistent.yaml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		_, err := Load("testdata/invalid.yaml")
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})
}

func TestLoadOverrides(t *testing.T) {
	t.Run("valid overrides directory", func(t *testing.T) {
		overrides, err := LoadOverrides("testdata/overrides")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(overrides) != 2 {
			t.Fatalf("overrides count = %d, want %d", len(overrides), 2)
		}

		svc1, ok := overrides["test-service-1"]
		if !ok {
			t.Fatal("missing override for test-service-1")
		}
		if len(svc1.Rules) != 2 {
			t.Errorf("test-service-1 rules count = %d, want %d", len(svc1.Rules), 2)
		}
		if _, ok := svc1.Rules["codeowners"]; !ok {
			t.Error("test-service-1 missing codeowners rule override")
		}
		if _, ok := svc1.Rules["repo-settings"]; !ok {
			t.Error("test-service-1 missing repo-settings rule override")
		}

		svc2, ok := overrides["test-service-2"]
		if !ok {
			t.Fatal("missing override for test-service-2")
		}
		if len(svc2.Rules) != 1 {
			t.Errorf("test-service-2 rules count = %d, want %d", len(svc2.Rules), 1)
		}
	})

	t.Run("nonexistent directory returns empty map", func(t *testing.T) {
		overrides, err := LoadOverrides("testdata/no-such-dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(overrides) != 0 {
			t.Errorf("overrides count = %d, want %d", len(overrides), 0)
		}
	})

	t.Run("path is a file not a directory", func(t *testing.T) {
		_, err := LoadOverrides("testdata/valid.yaml")
		if err == nil {
			t.Fatal("expected error when path is a file")
		}
	})

	t.Run("stat error that is not IsNotExist", func(t *testing.T) {
		// Traversing through a file (file/child) gives ENOTDIR, not IsNotExist.
		_, err := LoadOverrides("testdata/valid.yaml/child")
		if err == nil {
			t.Fatal("expected error for stat through a file")
		}
	})

	t.Run("unreadable override file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test-service-3.yaml")
		if err := os.WriteFile(path, []byte("rules: {}"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(path, 0644) })

		_, err := LoadOverrides(dir)
		if err == nil {
			t.Fatal("expected error for unreadable override file")
		}
	})

	t.Run("invalid override yaml", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "test-service-4.yaml"), []byte(": [broken"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = LoadOverrides(dir)
		if err == nil {
			t.Fatal("expected error for invalid override YAML")
		}
	})

	t.Run("non-yaml files are ignored", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "test-service-6.yaml"), []byte("rules: {}"), 0644)
		os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not yaml"), 0644)
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello"), 0644)

		overrides, err := LoadOverrides(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(overrides) != 1 {
			t.Errorf("overrides count = %d, want %d (only .yaml files)", len(overrides), 1)
		}
	})
}
