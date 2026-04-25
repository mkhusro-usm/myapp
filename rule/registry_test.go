package rule

import (
	"context"
	"testing"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

type fakeRepoRule struct{ name string }

func (f *fakeRepoRule) Name() string { return f.name }
func (f *fakeRepoRule) Evaluate(_ context.Context, _ *gh.Repository) (*Result, error) {
	return nil, nil
}
func (f *fakeRepoRule) Apply(_ context.Context, _ *gh.Repository) (*Result, error) {
	return nil, nil
}

type fakeOrgRule struct{ name string }

func (f *fakeOrgRule) Name() string                                { return f.name }
func (f *fakeOrgRule) Evaluate(_ context.Context) (*Result, error) { return nil, nil }
func (f *fakeOrgRule) Apply(_ context.Context) (*Result, error)    { return nil, nil }

func TestRegistry(t *testing.T) {
	t.Run("register and retrieve repo rules", func(t *testing.T) {
		reg := NewRegistry()
		reg.RegisterRepoRule(&fakeRepoRule{name: "rulesets"})
		reg.RegisterRepoRule(&fakeRepoRule{name: "codeowners"})

		rules := reg.RepoRules()
		if len(rules) != 2 {
			t.Fatalf("repo rules count = %d, want 2", len(rules))
		}
		if _, ok := rules["rulesets"]; !ok {
			t.Error("missing rulesets rule")
		}
		if _, ok := rules["codeowners"]; !ok {
			t.Error("missing codeowners rule")
		}
	})

	t.Run("register and retrieve org rules", func(t *testing.T) {
		reg := NewRegistry()
		reg.RegisterOrgRule(&fakeOrgRule{name: "org-rulesets"})

		rules := reg.OrgRules()
		if len(rules) != 1 {
			t.Fatalf("org rules count = %d, want 1", len(rules))
		}
		if _, ok := rules["org-rulesets"]; !ok {
			t.Error("missing org-rulesets rule")
		}
	})

	t.Run("empty registry", func(t *testing.T) {
		reg := NewRegistry()
		if len(reg.RepoRules()) != 0 {
			t.Errorf("repo rules count = %d, want 0", len(reg.RepoRules()))
		}
		if len(reg.OrgRules()) != 0 {
			t.Errorf("org rules count = %d, want 0", len(reg.OrgRules()))
		}
	})

	t.Run("same name overwrites", func(t *testing.T) {
		reg := NewRegistry()
		reg.RegisterRepoRule(&fakeRepoRule{name: "rulesets"})
		reg.RegisterRepoRule(&fakeRepoRule{name: "rulesets"})

		if len(reg.RepoRules()) != 1 {
			t.Errorf("repo rules count = %d, want 1 (overwrite)", len(reg.RepoRules()))
		}
	})
}
