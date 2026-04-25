package rule

import (
	"errors"
	"fmt"
	"testing"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

func TestRepoSettingsName(t *testing.T) {
	rs := NewRepoSettings(&stubRepoSettingsClient{}, RepoSettingsConfig{})
	if rs.Name() != "repo-settings" {
		t.Errorf("Name() = %q, want %q", rs.Name(), "repo-settings")
	}
}

func TestRepoSettingsCheck(t *testing.T) {
	t.Run("all compliant", func(t *testing.T) {
		rs := NewRepoSettings(nil, RepoSettingsConfig{
			AllowSquashMerge:    boolPtr(true),
			AllowRebaseMerge:    boolPtr(false),
			DeleteBranchOnMerge: boolPtr(true),
		})
		current := &gh.RepoSettings{
			AllowSquashMerge:    boolPtr(true),
			AllowRebaseMerge:    boolPtr(false),
			DeleteBranchOnMerge: boolPtr(true),
		}
		violations := rs.check(current)
		if len(violations) != 0 {
			t.Errorf("violations count = %d, want 0: %v", len(violations), violations)
		}
	})

	t.Run("bool drift", func(t *testing.T) {
		rs := NewRepoSettings(nil, RepoSettingsConfig{
			AllowMergeCommit: boolPtr(false),
			AllowSquashMerge: boolPtr(true),
		})
		current := &gh.RepoSettings{
			AllowMergeCommit: boolPtr(true),
			AllowSquashMerge: boolPtr(false),
		}
		violations := rs.check(current)
		if len(violations) != 2 {
			t.Fatalf("violations count = %d, want 2: %v", len(violations), violations)
		}
		if violations[0].Field != "allow-merge-commit" {
			t.Errorf("violations[0].Field = %q, want %q", violations[0].Field, "allow-merge-commit")
		}
	})

	t.Run("string drift", func(t *testing.T) {
		rs := NewRepoSettings(nil, RepoSettingsConfig{
			SquashMergeCommitTitle: strPtr("PR_TITLE"),
		})
		current := &gh.RepoSettings{
			SquashMergeCommitTitle: strPtr("COMMIT_OR_PR_TITLE"),
		}
		violations := rs.check(current)
		if len(violations) != 1 {
			t.Fatalf("violations count = %d, want 1", len(violations))
		}
		if violations[0].Expected != "PR_TITLE" {
			t.Errorf("Expected = %q, want %q", violations[0].Expected, "PR_TITLE")
		}
	})

	t.Run("unmanaged settings produce violations", func(t *testing.T) {
		rs := NewRepoSettings(nil, RepoSettingsConfig{})
		current := &gh.RepoSettings{
			AllowAutoMerge:     boolPtr(true),
			MergeCommitMessage: strPtr("PR_BODY"),
		}
		violations := rs.check(current)
		if len(violations) != 2 {
			t.Fatalf("violations count = %d, want 2: %v", len(violations), violations)
		}
		for _, v := range violations {
			if v.Message == "" {
				t.Error("expected unmanaged violation to have a message")
			}
		}
	})

	t.Run("nil current values ignored", func(t *testing.T) {
		rs := NewRepoSettings(nil, RepoSettingsConfig{
			AllowAutoMerge: boolPtr(true),
		})
		current := &gh.RepoSettings{}
		violations := rs.check(current)
		if len(violations) != 0 {
			t.Errorf("violations count = %d, want 0 (both nil/expected can't compare)", len(violations))
		}
	})
}

func TestRepoSettingsDesired(t *testing.T) {
	rs := NewRepoSettings(nil, RepoSettingsConfig{
		AllowSquashMerge:       boolPtr(true),
		DeleteBranchOnMerge:    boolPtr(true),
		SquashMergeCommitTitle: strPtr("PR_TITLE"),
	})
	d := rs.desired()
	if d.AllowSquashMerge == nil || *d.AllowSquashMerge != true {
		t.Errorf("AllowSquashMerge = %v, want true", d.AllowSquashMerge)
	}
	if d.DeleteBranchOnMerge == nil || *d.DeleteBranchOnMerge != true {
		t.Errorf("DeleteBranchOnMerge = %v, want true", d.DeleteBranchOnMerge)
	}
	if d.SquashMergeCommitTitle == nil || *d.SquashMergeCommitTitle != "PR_TITLE" {
		t.Errorf("SquashMergeCommitTitle = %v, want PR_TITLE", d.SquashMergeCommitTitle)
	}
	if d.AllowMergeCommit != nil {
		t.Errorf("AllowMergeCommit should be nil (unmanaged), got %v", d.AllowMergeCommit)
	}
}

func TestRepoSettingsEvaluate(t *testing.T) {
	t.Run("compliant", func(t *testing.T) {
		stub := &stubRepoSettingsClient{
			settings: &gh.RepoSettings{AllowSquashMerge: boolPtr(true)},
		}
		rs := NewRepoSettings(stub, RepoSettingsConfig{AllowSquashMerge: boolPtr(true)})

		result, err := rs.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compliant {
			t.Error("expected compliant result")
		}
	})

	t.Run("non-compliant", func(t *testing.T) {
		stub := &stubRepoSettingsClient{
			settings: &gh.RepoSettings{AllowSquashMerge: boolPtr(false)},
		}
		rs := NewRepoSettings(stub, RepoSettingsConfig{AllowSquashMerge: boolPtr(true)})

		result, err := rs.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compliant {
			t.Error("expected non-compliant result")
		}
	})

	t.Run("client error", func(t *testing.T) {
		stub := &stubRepoSettingsClient{getErr: errors.New("api error")}
		rs := NewRepoSettings(stub, RepoSettingsConfig{})

		_, err := rs.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRepoSettingsApply(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stub := &stubRepoSettingsClient{}
		rs := NewRepoSettings(stub, RepoSettingsConfig{AllowSquashMerge: boolPtr(true)})

		result, err := rs.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Applied {
			t.Error("expected Applied = true")
		}
		if !result.Compliant {
			t.Error("expected Compliant = true after apply")
		}
	})

	t.Run("client error", func(t *testing.T) {
		stub := &stubRepoSettingsClient{updateErr: fmt.Errorf("forbidden")}
		rs := NewRepoSettings(stub, RepoSettingsConfig{})

		_, err := rs.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
