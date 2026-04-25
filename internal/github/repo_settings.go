package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v84/github"
)

// RepoSettings represents the pull request and merge settings for a repository.
// Pointer fields allow distinguishing between "not set" (nil) and an explicit value,
// which is needed for partial updates via the GitHub API.
type RepoSettings struct {
	AllowMergeCommit         *bool   `json:"allow_merge_commit,omitempty"`
	AllowSquashMerge         *bool   `json:"allow_squash_merge,omitempty"`
	AllowRebaseMerge         *bool   `json:"allow_rebase_merge,omitempty"`
	AllowAutoMerge           *bool   `json:"allow_auto_merge,omitempty"`
	DeleteBranchOnMerge      *bool   `json:"delete_branch_on_merge,omitempty"`
	AllowUpdateBranch        *bool   `json:"allow_update_branch,omitempty"`
	SquashMergeCommitTitle   *string `json:"squash_merge_commit_title,omitempty"`
	SquashMergeCommitMessage *string `json:"squash_merge_commit_message,omitempty"`
	MergeCommitTitle         *string `json:"merge_commit_title,omitempty"`
	MergeCommitMessage       *string `json:"merge_commit_message,omitempty"`
}

// GetRepoSettings fetches the pull request and merge settings for a repository.
func (c *Client) GetRepoSettings(ctx context.Context, repoName string) (*RepoSettings, error) {
	repo, resp, err := c.restClient.Repositories.Get(ctx, c.org, repoName)
	if err != nil {
		return nil, fmt.Errorf("getting repository %s: %w", repoName, err)
	}
	logRateLimit(resp)

	return &RepoSettings{
		AllowMergeCommit:         repo.AllowMergeCommit,
		AllowSquashMerge:         repo.AllowSquashMerge,
		AllowRebaseMerge:         repo.AllowRebaseMerge,
		AllowAutoMerge:           repo.AllowAutoMerge,
		DeleteBranchOnMerge:      repo.DeleteBranchOnMerge,
		AllowUpdateBranch:        repo.AllowUpdateBranch,
		SquashMergeCommitTitle:   repo.SquashMergeCommitTitle,
		SquashMergeCommitMessage: repo.SquashMergeCommitMessage,
		MergeCommitTitle:         repo.MergeCommitTitle,
		MergeCommitMessage:       repo.MergeCommitMessage,
	}, nil
}

// UpdateRepoSettings applies the desired pull request and merge settings to a repository.
// Only non-nil fields are sent to the API; nil fields leave the current value unchanged.
func (c *Client) UpdateRepoSettings(ctx context.Context, repoName string, s *RepoSettings) error {
	_, resp, err := c.restClient.Repositories.Edit(ctx, c.org, repoName, &gogithub.Repository{
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
	logRateLimit(resp)

	return nil
}
