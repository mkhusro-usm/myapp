package rule

import (
	"context"
	"fmt"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// RepoSettingsConfig mirrors the desired repository PR/merge settings from config.
type RepoSettingsConfig struct {
	AllowMergeCommit         bool   `yaml:"allow-merge-commit"`
	AllowSquashMerge         bool   `yaml:"allow-squash-merge"`
	AllowRebaseMerge         bool   `yaml:"allow-rebase-merge"`
	AllowAutoMerge           bool   `yaml:"allow-auto-merge"`
	DeleteBranchOnMerge      bool   `yaml:"delete-branch-on-merge"`
	AllowUpdateBranch        bool   `yaml:"allow-update-branch"`
	SquashMergeCommitTitle   string `yaml:"squash-merge-commit-title"`
	SquashMergeCommitMessage string `yaml:"squash-merge-commit-message"`
	MergeCommitTitle         string `yaml:"merge-commit-title"`
	MergeCommitMessage       string `yaml:"merge-commit-message"`
}

type RepoSettings struct {
	client   *gh.Client
	settings RepoSettingsConfig
}

func NewRepoSettings(client *gh.Client, settings RepoSettingsConfig) *RepoSettings {
	return &RepoSettings{client: client, settings: settings}
}

func (rs *RepoSettings) Name() string {
	return "repo_settings"
}

func (rs *RepoSettings) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	current, err := rs.client.GetRepoSettings(ctx, repo.Name)
	if err != nil {
		return nil, fmt.Errorf("fetching repo settings for %s: %w", repo.FullName(), err)
	}

	return NewResult(rs.Name(), repo.FullName(), rs.check(current)), nil
}

func (rs *RepoSettings) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	desired := rs.desiredSettings()

	if err := rs.client.UpdateRepoSettings(ctx, repo.Name, desired); err != nil {
		return nil, fmt.Errorf("applying repo settings for %s: %w", repo.FullName(), err)
	}

	r := NewResult(rs.Name(), repo.FullName(), nil)
	r.Applied = true
	return r, nil
}

func (rs *RepoSettings) desiredSettings() *gh.RepoSettings {
	return &gh.RepoSettings{
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

	addBool := func(field string, expected, actual bool) {
		if expected != actual {
			violations = append(violations, Violation{
				Field:    field,
				Expected: fmt.Sprintf("%t", expected),
				Actual:   fmt.Sprintf("%t", actual),
			})
		}
	}

	addString := func(field, expected, actual string) {
		if expected != "" && expected != actual {
			violations = append(violations, Violation{
				Field:    field,
				Expected: expected,
				Actual:   actual,
			})
		}
	}

	addBool("allow-merge-commit", rs.settings.AllowMergeCommit, current.AllowMergeCommit)
	addBool("allow-squash-merge", rs.settings.AllowSquashMerge, current.AllowSquashMerge)
	addBool("allow-rebase-merge", rs.settings.AllowRebaseMerge, current.AllowRebaseMerge)
	addBool("allow-auto-merge", rs.settings.AllowAutoMerge, current.AllowAutoMerge)
	addBool("delete-branch-on-merge", rs.settings.DeleteBranchOnMerge, current.DeleteBranchOnMerge)
	addBool("allow-update-branch", rs.settings.AllowUpdateBranch, current.AllowUpdateBranch)

	addString("squash-merge-commit-title", rs.settings.SquashMergeCommitTitle, current.SquashMergeCommitTitle)
	addString("squash-merge-commit-message", rs.settings.SquashMergeCommitMessage, current.SquashMergeCommitMessage)
	addString("merge-commit-title", rs.settings.MergeCommitTitle, current.MergeCommitTitle)
	addString("merge-commit-message", rs.settings.MergeCommitMessage, current.MergeCommitMessage)

	return violations
}
