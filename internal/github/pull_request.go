package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v84/github"
)

const refsHeadsPrefix = "refs/heads/"

// FileChange represents a file to be created or updated in a repository.
type FileChange struct {
	Path    string // File path within the repository (e.g., ".github/CODEOWNERS")
	Content []byte // File content
	SHA     string // Object SHA for updates (optional, filled internally)
}

// CreateFileChangePR creates a branch, commits file changes, and opens a pull request.
// The commit is always parented on the current base branch SHA, and the branch is
// force-pushed to the new commit. This ensures deterministic, idempotent runs.
func (c *Client) CreateFileChangePR(ctx context.Context, repoName, baseBranch, branchName, prTitle, prBody string, changes []FileChange) (string, error) {
	baseSHA, err := c.ensureBranch(ctx, repoName, branchName, baseBranch)
	if err != nil {
		return "", fmt.Errorf("preparing branch: %w", err)
	}

	newCommitSHA, err := c.createCommitForChanges(ctx, repoName, baseSHA, prTitle, changes)
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}

	if err := c.forceUpdateRef(ctx, repoName, branchName, newCommitSHA); err != nil {
		return "", fmt.Errorf("updating branch ref: %w", err)
	}

	pr, err := c.getOrCreatePullRequest(ctx, repoName, branchName, baseBranch, prTitle, prBody)
	if err != nil {
		return "", fmt.Errorf("preparing PR: %w", err)
	}

	return pr.GetHTMLURL(), nil
}

// ensureBranch ensures the target branch exists (creating it if needed) and returns
// the base branch SHA. Always returns the base SHA so callers can reset and commit
// on top of the current base, regardless of the target branch's prior state.
func (c *Client) ensureBranch(ctx context.Context, repoName, branchName, baseBranch string) (string, error) {
	baseRef, resp, err := c.restClient.Git.GetRef(ctx, c.org, repoName, refsHeadsPrefix+baseBranch)
	if err != nil {
		return "", fmt.Errorf("getting base branch %s: %w", baseBranch, err)
	}
	logRateLimit(resp)
	baseSHA := baseRef.GetObject().GetSHA()

	_, resp, err = c.restClient.Git.CreateRef(ctx, c.org, repoName, gogithub.CreateRef{
		Ref: refsHeadsPrefix + branchName,
		SHA: baseSHA,
	})
	if err != nil {
		// Branch may already exist — verify it's reachable.
		_, resp, err = c.restClient.Git.GetRef(ctx, c.org, repoName, refsHeadsPrefix+branchName)
		if err != nil {
			return "", fmt.Errorf("ensuring branch %s exists: %w", branchName, err)
		}
	}
	logRateLimit(resp)

	return baseSHA, nil
}

// getOrCreatePullRequest returns an existing open PR, or creates a new one.
// It searches for an open PR with the given head and base branches.
func (c *Client) getOrCreatePullRequest(ctx context.Context, repoName, head, base, title, body string) (*gogithub.PullRequest, error) {
	page := 1
	for {
		prs, resp, err := c.restClient.PullRequests.List(ctx, c.org, repoName, &gogithub.PullRequestListOptions{
			State: "open",
			Head:  head,
			Base:  base,
			ListOptions: gogithub.ListOptions{
				Page: page,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("listing pull requests: %w", err)
		}
		logRateLimit(resp)

		for _, pr := range prs {
			if pr.GetHead().GetRef() == head && pr.GetBase().GetRef() == base {
				return pr, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	pr, resp, err := c.restClient.PullRequests.Create(ctx, c.org, repoName, &gogithub.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return nil, fmt.Errorf("creating pull request: %w", err)
	}
	logRateLimit(resp)
	return pr, nil
}

// createCommitForChanges creates a single commit with all file changes.
// It builds: blobs → tree → commit.
func (c *Client) createCommitForChanges(ctx context.Context, repoName string, baseSHA string, commitMsg string, changes []FileChange) (string, error) {
	baseCommit, resp, err := c.restClient.Git.GetCommit(ctx, c.org, repoName, baseSHA)
	if err != nil {
		return "", fmt.Errorf("getting base commit: %w", err)
	}
	logRateLimit(resp)

	var entries []*gogithub.TreeEntry
	for _, change := range changes {
		blob, resp, err := c.restClient.Git.CreateBlob(ctx, c.org, repoName, gogithub.Blob{
			Content:  gogithub.Ptr(string(change.Content)),
			Encoding: gogithub.Ptr("utf-8"),
		})
		if err != nil {
			return "", fmt.Errorf("creating blob for %s: %w", change.Path, err)
		}
		logRateLimit(resp)

		entries = append(entries, &gogithub.TreeEntry{
			Path: gogithub.Ptr(change.Path),
			Mode: gogithub.Ptr("100644"),
			Type: gogithub.Ptr("blob"),
			SHA:  blob.SHA,
		})
	}

	treeSHA := baseCommit.GetTree().GetSHA()
	tree, resp, err := c.restClient.Git.CreateTree(ctx, c.org, repoName, treeSHA, entries)
	if err != nil {
		return "", fmt.Errorf("creating tree: %w", err)
	}
	logRateLimit(resp)

	commit, resp, err := c.restClient.Git.CreateCommit(ctx, c.org, repoName, gogithub.Commit{
		Message: gogithub.Ptr(commitMsg),
		Tree:    tree,
		Parents: []*gogithub.Commit{
			{SHA: gogithub.Ptr(baseSHA)},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}
	logRateLimit(resp)

	return commit.GetSHA(), nil
}

// forceUpdateRef force-updates the branch to point at the given SHA.
func (c *Client) forceUpdateRef(ctx context.Context, repoName, branchName, sha string) error {
	force := true
	_, resp, err := c.restClient.Git.UpdateRef(ctx, c.org, repoName, refsHeadsPrefix+branchName, gogithub.UpdateRef{
		SHA:   sha,
		Force: &force,
	})
	logRateLimit(resp)
	return err
}
