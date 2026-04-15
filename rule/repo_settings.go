package rule

import (
	"context"
	"fmt"
	"log"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// RepoSettingsConfig mirrors the desired repository PR/merge settings from config.
// Pointer fields are optional — only non-nil fields are evaluated and applied.
type RepoSettingsConfig struct {
	AllowMergeCommit         *bool   `yaml:"allow-merge-commit"`
	AllowSquashMerge         *bool   `yaml:"allow-squash-merge"`
	AllowRebaseMerge         *bool   `yaml:"allow-rebase-merge"`
	AllowAutoMerge           *bool   `yaml:"allow-auto-merge"`
	DeleteBranchOnMerge      *bool   `yaml:"delete-branch-on-merge"`
	AllowUpdateBranch        *bool   `yaml:"allow-update-branch"`
	SquashMergeCommitTitle   *string `yaml:"squash-merge-commit-title"`
	SquashMergeCommitMessage *string `yaml:"squash-merge-commit-message"`
	MergeCommitTitle         *string `yaml:"merge-commit-title"`
	MergeCommitMessage       *string `yaml:"merge-commit-message"`
}

// RepoSettings enforces repository-level PR and merge settings.
type RepoSettings struct {
	client   *gh.Client
	settings RepoSettingsConfig
}

// NewRepoSettings creates a RepoSettings rule with the given settings.
func NewRepoSettings(client *gh.Client, settings RepoSettingsConfig) *RepoSettings {
	return &RepoSettings{client: client, settings: settings}
}

func (rs *RepoSettings) Name() string {
	return "repo-settings"
}

// Evaluate checks whether the repository's PR/merge settings match the config.
// Only settings that are explicitly configured are checked.
func (rs *RepoSettings) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] evaluating repository PR/merge settings", repo.FullName())
	current, err := rs.client.GetRepoSettings(ctx, repo.Name)
	if err != nil {
		return nil, fmt.Errorf("fetching repo settings for %s: %w", repo.FullName(), err)
	}

	return NewResult(rs.Name(), repo.FullName(), rs.check(current)), nil
}

// Apply sets the configured PR/merge settings on the repository.
// Only settings that are explicitly configured are applied.
func (rs *RepoSettings) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] applying repository PR/merge settings", repo.FullName())
	if err := rs.client.UpdateRepoSettings(ctx, repo.Name, rs.desiredInput()); err != nil {
		return nil, fmt.Errorf("applying repo settings for %s: %w", repo.FullName(), err)
	}

	r := NewResult(rs.Name(), repo.FullName(), nil)
	r.Applied = true
	
	return r, nil
}

func (rs *RepoSettings) desiredInput() *gh.RepoSettingsInput {
	return &gh.RepoSettingsInput{
		AllowMergeCommit:         rs.settings.AllowMergeCommit,
		AllowSquashMerge:         rs.settings.AllowSquashMerge,
		AllowRebaseMerge:         rs.settings.AllowRebaseMerge,
		AllowAutoMerge:           rs.settings.AllowAutoMerge,
		DeleteBranchOnMerge:      rs.settings.DeleteBranchOnMerge,
		AllowUpdateBranch:        rs.settings.AllowUpdateBranch,
		SquashMergeCommitTitle:   rs.settings.SquashMergeCommitTitle,
		SquashMergeCommitMessage: rs.settings.SquashMergeCommitMessage,
		MergeCommitTitle:         rs.settings.MergeCommitTitle,
		MergeCommitMessage:       rs.settings.MergeCommitMessage,
	}
}

func (rs *RepoSettings) check(current *gh.RepoSettings) []Violation {
	var violations []Violation

	checkBool := func(field string, expected *bool, actual bool) {
		if expected != nil && *expected != actual {
			violations = append(violations, Violation{
				Field:    field,
				Expected: fmt.Sprintf("%t", *expected),
				Actual:   fmt.Sprintf("%t", actual),
			})
		}
	}

	checkString := func(field string, expected *string, actual string) {
		if expected != nil && *expected != actual {
			violations = append(violations, Violation{
				Field:    field,
				Expected: *expected,
				Actual:   actual,
			})
		}
	}

	checkBool("allow-merge-commit", rs.settings.AllowMergeCommit, current.AllowMergeCommit)
	checkBool("allow-squash-merge", rs.settings.AllowSquashMerge, current.AllowSquashMerge)
	checkBool("allow-rebase-merge", rs.settings.AllowRebaseMerge, current.AllowRebaseMerge)
	checkBool("allow-auto-merge", rs.settings.AllowAutoMerge, current.AllowAutoMerge)
	checkBool("delete-branch-on-merge", rs.settings.DeleteBranchOnMerge, current.DeleteBranchOnMerge)
	checkBool("allow-update-branch", rs.settings.AllowUpdateBranch, current.AllowUpdateBranch)

	checkString("squash-merge-commit-title", rs.settings.SquashMergeCommitTitle, current.SquashMergeCommitTitle)
	checkString("squash-merge-commit-message", rs.settings.SquashMergeCommitMessage, current.SquashMergeCommitMessage)
	checkString("merge-commit-title", rs.settings.MergeCommitTitle, current.MergeCommitTitle)
	checkString("merge-commit-message", rs.settings.MergeCommitMessage, current.MergeCommitMessage)

	return violations
}
