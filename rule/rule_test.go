package rule

import (
	"testing"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

func TestNewResult(t *testing.T) {
	t.Run("compliant when no violations", func(t *testing.T) {
		r := NewResult("test-rule", "org/repo", nil)
		if !r.Compliant {
			t.Error("expected Compliant = true")
		}
		if r.ViolationCount != 0 {
			t.Errorf("ViolationCount = %d, want 0", r.ViolationCount)
		}
		if r.RuleName != "test-rule" {
			t.Errorf("RuleName = %q, want %q", r.RuleName, "test-rule")
		}
		if r.Repository != "org/repo" {
			t.Errorf("Repository = %q, want %q", r.Repository, "org/repo")
		}
	})

	t.Run("non-compliant with violations", func(t *testing.T) {
		violations := []Violation{
			{Field: "a", Expected: "true", Actual: "false"},
			{Field: "b", Expected: "1", Actual: "2"},
		}
		r := NewResult("test-rule", "org/repo", violations)
		if r.Compliant {
			t.Error("expected Compliant = false")
		}
		if r.ViolationCount != 2 {
			t.Errorf("ViolationCount = %d, want 2", r.ViolationCount)
		}
	})
}

func TestParseSettings(t *testing.T) {
	t.Run("valid settings", func(t *testing.T) {
		raw := map[string]any{
			"entries": []any{
				map[string]any{"pattern": "*", "owners": []any{"@team"}},
			},
		}
		settings, err := ParseSettings[CodeownersSettings](raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(settings.Entries) != 1 {
			t.Fatalf("entries count = %d, want 1", len(settings.Entries))
		}
		if settings.Entries[0].Pattern != "*" {
			t.Errorf("pattern = %q, want %q", settings.Entries[0].Pattern, "*")
		}
	})

	t.Run("empty map", func(t *testing.T) {
		settings, err := ParseSettings[CodeownersSettings](map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(settings.Entries) != 0 {
			t.Errorf("entries count = %d, want 0", len(settings.Entries))
		}
	})
}

func TestDefaultBranch(t *testing.T) {
	t.Run("uses repo branch", func(t *testing.T) {
		repo := &gh.Repository{DefaultBranch: "develop"}
		if got := defaultBranch(repo); got != "develop" {
			t.Errorf("defaultBranch = %q, want %q", got, "develop")
		}
	})

	t.Run("falls back to main", func(t *testing.T) {
		repo := &gh.Repository{}
		if got := defaultBranch(repo); got != "main" {
			t.Errorf("defaultBranch = %q, want %q", got, "main")
		}
	})
}
