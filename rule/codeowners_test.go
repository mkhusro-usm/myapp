package rule

import (
	"errors"
	"testing"

	"github.com/mkhusro-usm/myapp/config"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

func TestCodeownersName(t *testing.T) {
	co := NewCodeowners(&stubCodeownersClient{}, CodeownersSettings{}, nil)
	if co.Name() != "codeowners" {
		t.Errorf("Name() = %q, want %q", co.Name(), "codeowners")
	}
}

func TestBuildContent(t *testing.T) {
	settings := CodeownersSettings{
		Entries: []CodeownersEntry{
			{Pattern: "*", Owners: []string{"@platform-team"}},
			{Pattern: "/docs", Owners: []string{"@docs-team", "@writers"}},
		},
	}
	got := buildContent(settings)
	want := "# CODEOWNERS — managed by governance tool\n# Do not edit manually; changes will be overwritten.\n\n* @platform-team\n/docs @docs-team @writers\n"
	if got != want {
		t.Errorf("buildContent =\n%q\nwant:\n%q", got, want)
	}
}

func TestCodeownersCheck(t *testing.T) {
	co := NewCodeowners(nil, CodeownersSettings{}, nil)

	t.Run("missing file", func(t *testing.T) {
		violations := co.check("", CodeownersSettings{
			Entries: []CodeownersEntry{{Pattern: "*", Owners: []string{"@team"}}},
		})
		if len(violations) != 1 {
			t.Fatalf("violations count = %d, want 1", len(violations))
		}
		if violations[0].Actual != "missing" {
			t.Errorf("Actual = %q, want %q", violations[0].Actual, "missing")
		}
	})

	t.Run("all entries present", func(t *testing.T) {
		content := "* @platform-team\n/docs @docs-team\n"
		settings := CodeownersSettings{
			Entries: []CodeownersEntry{
				{Pattern: "*", Owners: []string{"@platform-team"}},
				{Pattern: "/docs", Owners: []string{"@docs-team"}},
			},
		}
		violations := co.check(content, settings)
		if len(violations) != 0 {
			t.Errorf("violations count = %d, want 0: %v", len(violations), violations)
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		content := "* @platform-team\n"
		settings := CodeownersSettings{
			Entries: []CodeownersEntry{
				{Pattern: "*", Owners: []string{"@platform-team"}},
				{Pattern: "/docs", Owners: []string{"@docs-team"}},
			},
		}
		violations := co.check(content, settings)
		if len(violations) != 1 {
			t.Fatalf("violations count = %d, want 1", len(violations))
		}
		if violations[0].Expected != "/docs @docs-team" {
			t.Errorf("Expected = %q, want %q", violations[0].Expected, "/docs @docs-team")
		}
	})

	t.Run("unexpected entry", func(t *testing.T) {
		content := "* @platform-team\n/secret @rogue-team\n"
		settings := CodeownersSettings{
			Entries: []CodeownersEntry{
				{Pattern: "*", Owners: []string{"@platform-team"}},
			},
		}
		violations := co.check(content, settings)
		if len(violations) != 1 {
			t.Fatalf("violations count = %d, want 1: %v", len(violations), violations)
		}
		if violations[0].Actual != "/secret @rogue-team" {
			t.Errorf("Actual = %q, want unexpected entry", violations[0].Actual)
		}
	})

	t.Run("comments and blank lines ignored", func(t *testing.T) {
		content := "# managed by governance\n\n* @platform-team\n\n"
		settings := CodeownersSettings{
			Entries: []CodeownersEntry{
				{Pattern: "*", Owners: []string{"@platform-team"}},
			},
		}
		violations := co.check(content, settings)
		if len(violations) != 0 {
			t.Errorf("violations count = %d, want 0: %v", len(violations), violations)
		}
	})
}

func TestEffectiveSettings(t *testing.T) {
	t.Run("no override uses baseline", func(t *testing.T) {
		co := &Codeowners{
			settings: CodeownersSettings{
				Entries: []CodeownersEntry{{Pattern: "*", Owners: []string{"@team"}}},
			},
		}
		eff := co.effectiveSettings("repo-a")
		if len(eff.Entries) != 1 {
			t.Fatalf("entries count = %d, want 1", len(eff.Entries))
		}
	})

	t.Run("with override merges entries", func(t *testing.T) {
		co := &Codeowners{
			settings: CodeownersSettings{
				Entries: []CodeownersEntry{{Pattern: "*", Owners: []string{"@team"}}},
			},
			overrides: map[string]CodeownersSettings{
				"repo-a": {Entries: []CodeownersEntry{{Pattern: "/docs", Owners: []string{"@docs"}}}},
			},
		}
		eff := co.effectiveSettings("repo-a")
		if len(eff.Entries) != 2 {
			t.Fatalf("entries count = %d, want 2", len(eff.Entries))
		}
	})
}

func TestParseCodeownersOverrides(t *testing.T) {
	baseline := CodeownersSettings{
		Entries: []CodeownersEntry{{Pattern: "*", Owners: []string{"@team"}}},
	}

	t.Run("valid override", func(t *testing.T) {
		raw := map[string]config.RepoOverride{
			"repo-a": {Rules: map[string]config.RepoOverrideRule{
				"codeowners": {Settings: map[string]any{
					"entries": []any{
						map[string]any{"pattern": "/docs", "owners": []any{"@docs"}},
					},
				}},
			}},
		}
		result := parseCodeownersOverrides(baseline, raw)
		if len(result) != 1 {
			t.Fatalf("overrides count = %d, want 1", len(result))
		}
		if len(result["repo-a"].Entries) != 1 {
			t.Errorf("repo-a entries = %d, want 1", len(result["repo-a"].Entries))
		}
	})

	t.Run("conflicting pattern skipped", func(t *testing.T) {
		raw := map[string]config.RepoOverride{
			"repo-a": {Rules: map[string]config.RepoOverrideRule{
				"codeowners": {Settings: map[string]any{
					"entries": []any{
						map[string]any{"pattern": "*", "owners": []any{"@rogue"}},
					},
				}},
			}},
		}
		result := parseCodeownersOverrides(baseline, raw)
		if result != nil {
			t.Errorf("expected nil overrides when all entries conflict, got %v", result)
		}
	})

	t.Run("no codeowners rule in override", func(t *testing.T) {
		raw := map[string]config.RepoOverride{
			"repo-a": {Rules: map[string]config.RepoOverrideRule{
				"repo-settings": {Settings: map[string]any{}},
			}},
		}
		result := parseCodeownersOverrides(baseline, raw)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("invalid override yaml", func(t *testing.T) {
		raw := map[string]config.RepoOverride{
			"repo-a": {Rules: map[string]config.RepoOverrideRule{
				"codeowners": {Settings: map[string]any{
					"entries": "not-a-list",
				}},
			}},
		}
		result := parseCodeownersOverrides(baseline, raw)
		if result != nil {
			t.Errorf("expected nil for invalid override, got %v", result)
		}
	})

	t.Run("nil overrides", func(t *testing.T) {
		result := parseCodeownersOverrides(baseline, nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
}

func TestCodeownersEvaluate(t *testing.T) {
	baseline := CodeownersSettings{
		Entries: []CodeownersEntry{{Pattern: "*", Owners: []string{"@platform"}}},
	}
	desired := buildContent(baseline)

	t.Run("compliant", func(t *testing.T) {
		stub := &stubCodeownersClient{content: desired}
		co := NewCodeowners(stub, baseline, nil)

		result, err := co.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compliant {
			t.Errorf("expected compliant, got violations: %v", result.Violations)
		}
	})

	t.Run("non-compliant", func(t *testing.T) {
		stub := &stubCodeownersClient{content: "wrong content\n"}
		co := NewCodeowners(stub, baseline, nil)

		result, err := co.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compliant {
			t.Error("expected non-compliant")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		stub := &stubCodeownersClient{getErr: errors.New("not found")}
		co := NewCodeowners(stub, baseline, nil)

		_, err := co.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCodeownersApply(t *testing.T) {
	baseline := CodeownersSettings{
		Entries: []CodeownersEntry{{Pattern: "*", Owners: []string{"@platform"}}},
	}
	desired := buildContent(baseline)

	t.Run("already compliant", func(t *testing.T) {
		stub := &stubCodeownersClient{content: desired}
		co := NewCodeowners(stub, baseline, nil)

		result, err := co.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Applied {
			t.Error("expected Applied = false when already compliant")
		}
	})

	t.Run("creates PR", func(t *testing.T) {
		stub := &stubCodeownersClient{
			content: "old content\n",
			prURL:   "https://github.com/org/repo-a/pull/1",
		}
		co := NewCodeowners(stub, baseline, nil)

		result, err := co.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Applied {
			t.Error("expected Applied = true")
		}
		if result.PullRequestURL != "https://github.com/org/repo-a/pull/1" {
			t.Errorf("PullRequestURL = %q, want PR URL", result.PullRequestURL)
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		stub := &stubCodeownersClient{getErr: errors.New("api error")}
		co := NewCodeowners(stub, baseline, nil)

		_, err := co.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("PR creation error", func(t *testing.T) {
		stub := &stubCodeownersClient{
			content: "old content\n",
			prErr:   errors.New("PR failed"),
		}
		co := NewCodeowners(stub, baseline, nil)

		_, err := co.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
