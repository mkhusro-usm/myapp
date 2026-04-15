package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v62/github"
)

// RepoSettings represents the pull request and merge settings for a repository.
type RepoSettings struct {
	AllowMergeCommit         bool
	AllowSquashMerge         bool
	AllowRebaseMerge         bool
	AllowAutoMerge           bool
	DeleteBranchOnMerge      bool
	AllowUpdateBranch        bool
	SquashMergeCommitTitle   string // "PR_TITLE" or "COMMIT_OR_PR_TITLE"
	SquashMergeCommitMessage string // "PR_BODY", "COMMIT_MESSAGES", or "BLANK"
	MergeCommitTitle         string // "PR_TITLE" or "MERGE_MESSAGE"
	MergeCommitMessage       string // "PR_BODY", "PR_TITLE", or "BLANK"
}

// GetRepoSettings fetches the pull request and merge settings for a repository.
func (c *Client) GetRepoSettings(ctx context.Context, repoName string) (*RepoSettings, error) {
	repo, _, err := c.REST.Repositories.Get(ctx, c.org, repoName)
	if err != nil {
		return nil, fmt.Errorf("getting repository %s: %w", repoName, err)
	}
	
	return &RepoSettings{
		AllowMergeCommit:         repo.GetAllowMergeCommit(),
		AllowSquashMerge:         repo.GetAllowSquashMerge(),
		AllowRebaseMerge:         repo.GetAllowRebaseMerge(),
		AllowAutoMerge:           repo.GetAllowAutoMerge(),
		DeleteBranchOnMerge:      repo.GetDeleteBranchOnMerge(),
		AllowUpdateBranch:        repo.GetAllowUpdateBranch(),
		SquashMergeCommitTitle:   repo.GetSquashMergeCommitTitle(),
		SquashMergeCommitMessage: repo.GetSquashMergeCommitMessage(),
		MergeCommitTitle:         repo.GetMergeCommitTitle(),
		MergeCommitMessage:       repo.GetMergeCommitMessage(),
	}, nil
}

// RepoSettingsInput represents the desired state for updates.
// Nil fields are not sent to the API and leave the current value unchanged.
type RepoSettingsInput struct {
	AllowMergeCommit         *bool
	AllowSquashMerge         *bool
	AllowRebaseMerge         *bool
	AllowAutoMerge           *bool
	DeleteBranchOnMerge      *bool
	AllowUpdateBranch        *bool
	SquashMergeCommitTitle   *string
	SquashMergeCommitMessage *string
	MergeCommitTitle         *string
	MergeCommitMessage       *string
}

// UpdateRepoSettings applies the desired pull request and merge settings to a repository.
// Only non-nil fields in the input are applied; nil fields are left unchanged.
func (c *Client) UpdateRepoSettings(ctx context.Context, repoName string, s *RepoSettingsInput) error {
	_, _, err := c.REST.Repositories.Edit(ctx, c.org, repoName, &gogithub.Repository{
		AllowMergeCommit:         s.AllowMergeCommit,
		AllowSquashMerge:         s.AllowSquashMerge,
		AllowRebaseMerge:         s.AllowRebaseMerge,
		AllowAutoMerge:           s.AllowAutoMerge,
		DeleteBranchOnMerge:      s.DeleteBranchOnMerge,
		AllowUpdateBranch:        s.AllowUpdateBranch,
		SquashMergeCommitTitle:   s.SquashMergeCommitTitle,
		SquashMergeCommitMessage: s.SquashMergeCommitMessage,
		MergeCommitTitle:         s.MergeCommitTitle,
		MergeCommitMessage:       s.MergeCommitMessage,
	})
	if err != nil {
		return fmt.Errorf("updating repository settings for %s: %w", repoName, err)
	}
	
	return nil
}
