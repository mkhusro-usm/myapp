package github

import (
	"context"
	"fmt"
	"net/http"

	gogithub "github.com/google/go-github/v84/github"
)

const refsHeadsPrefix = "refs/heads/"

// FileChange represents a file to be created or updated in a repository.
type FileChange struct {
	Path    string // File path within the repository (e.g., ".github/CODEOWNERS")
	Content []byte // File content
	SHA     string // Object SHA for updates (optional, filled internally)
}

// CreateFileChangePR creates a branch, commits multiple file changes, and opens a pull request.
// It uses a constant branch name (branchPrefix) — if the branch/PR already exists, it reuses them.
// Returns the PR's HTML URL. This is the standard workflow for governance rules that apply via PR.
func (c *Client) CreateFileChangePR(ctx context.Context, repoName, baseBranch, branchPrefix, prTitle, prBody string, changes []FileChange) (string, error) {
	// Get or create the branch.
	if _, err := c.getOrCreateBranch(ctx, repoName, branchPrefix, baseBranch); err != nil {
		return "", fmt.Errorf("preparing branch: %w", err)
	}

	// Phase 1: Get SHAs for all files.
	for i := range changes {
		sha, err := c.getFileSHA(ctx, repoName, branchPrefix, changes[i].Path)
		if err != nil {
			return "", fmt.Errorf("getting existing file SHA for %s: %w", changes[i].Path, err)
		}
		changes[i].SHA = sha
	}

	// Phase 2: Commit all files.
	for _, change := range changes {
		msg := fmt.Sprintf("Update %s", change.Path)
		if err := c.commitFile(ctx, repoName, branchPrefix, change.Path, msg, change.Content, change.SHA); err != nil {
			return "", fmt.Errorf("committing file %s: %w", change.Path, err)
		}
	}

	// Get or create the pull request.
	pr, err := c.getOrCreatePullRequest(ctx, repoName, branchPrefix, baseBranch, prTitle, prBody)
	if err != nil {
		return "", fmt.Errorf("preparing pull request: %w", err)
	}

	return pr.GetHTMLURL(), nil
}

// getOrCreateBranch returns the SHA of an existing branch, or creates a new branch from baseBranch.
// If the branch already exists, it returns the existing branch's SHA.
// If creation fails with "already exists", it fetches and returns the existing SHA.
func (c *Client) getOrCreateBranch(ctx context.Context, repoName, branchPrefix, baseBranch string) (string, error) {
	// Get SHA of base branch to create from.
	baseRef, _, err := c.restClient.Git.GetRef(ctx, c.org, repoName, refsHeadsPrefix+baseBranch)
	if err != nil {
		return "", fmt.Errorf("getting base branch %s: %w", baseBranch, err)
	}
	baseSHA := baseRef.GetObject().GetSHA()

	// Try to create the branch.
	_, _, err = c.restClient.Git.CreateRef(ctx, c.org, repoName, gogithub.CreateRef{
		Ref: refsHeadsPrefix + branchPrefix,
		SHA: baseSHA,
	})
	if err == nil {
		return baseSHA, nil
	}

	// If error is "Reference already exists" (409 Conflict), fetch and return existing SHA.
	if resp, ok := err.(*gogithub.ErrorResponse); ok && resp.Response.StatusCode == http.StatusConflict {
		ref, _, err := c.restClient.Git.GetRef(ctx, c.org, repoName, refsHeadsPrefix+branchPrefix)
		if err != nil {
			return "", fmt.Errorf("getting existing branch %s: %w", branchPrefix, err)
		}
		return ref.GetObject().GetSHA(), nil
	}

	return "", fmt.Errorf("creating branch %s: %w", branchPrefix, err)
}

// getOrCreatePullRequest returns an existing open PR, or creates a new one.
// It searches for an open PR with the given head and base branches.
func (c *Client) getOrCreatePullRequest(ctx context.Context, repoName, head, base, title, body string) (*gogithub.PullRequest, error) {
	// Search for existing open PR.
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

	// No existing PR found — create one.
	pr, _, err := c.restClient.PullRequests.Create(ctx, c.org, repoName, &gogithub.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return nil, fmt.Errorf("creating pull request: %w", err)
	}
	return pr, nil
}

// getFileSHA returns the blob SHA of a file on the given branch.
// Returns an empty string (without error) if the file does not exist.
func (c *Client) getFileSHA(ctx context.Context, repoName, branch, path string) (string, error) {
	opts := &gogithub.RepositoryContentGetOptions{Ref: branch}
	file, _, resp, err := c.restClient.Repositories.GetContents(ctx, c.org, repoName, path, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", nil
		}
		return "", fmt.Errorf("getting file SHA for %s: %w", path, err)
	}

	if file == nil {
		return "", nil
	}

	return file.GetSHA(), nil
}

// commitFile creates or updates a single file on the given branch.
// If blobSHA is non-empty the file is treated as an update; otherwise as a create.
func (c *Client) commitFile(ctx context.Context, repoName, branch, path, message string, content []byte, blobSHA string) error {
	opts := &gogithub.RepositoryContentFileOptions{
		Message: &message,
		Content: content,
		Branch:  &branch,
	}
	if blobSHA != "" {
		opts.SHA = &blobSHA
	}

	_, _, err := c.restClient.Repositories.CreateFile(ctx, c.org, repoName, path, opts)
	if err != nil {
		return fmt.Errorf("committing file %s: %w", path, err)
	}

	return nil
}
