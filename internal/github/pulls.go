package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gogithub "github.com/google/go-github/v62/github"
)

const refsHeadsPrefix = "refs/heads/"

// GetBranchSHA returns the commit SHA at the tip of the given branch.
func (c *Client) GetBranchSHA(ctx context.Context, repoName, branch string) (string, error) {
	ref, _, err := c.REST.Git.GetRef(ctx, c.org, repoName, refsHeadsPrefix+branch)
	if err != nil {
		return "", fmt.Errorf("getting ref for branch %s: %w", branch, err)
	}
	return ref.GetObject().GetSHA(), nil
}

// CreateBranch creates a new branch pointing at the given base SHA.
func (c *Client) CreateBranch(ctx context.Context, repoName, branchName, baseSHA string) error {
	ref := refsHeadsPrefix + branchName
	_, _, err := c.REST.Git.CreateRef(ctx, c.org, repoName, &gogithub.Reference{
		Ref:    &ref,
		Object: &gogithub.GitObject{SHA: &baseSHA},
	})
	if err != nil {
		return fmt.Errorf("creating branch %s: %w", branchName, err)
	}
	return nil
}

// GetFileSHA returns the blob SHA of a file on the given branch.
// Returns an empty string (without error) if the file does not exist.
func (c *Client) GetFileSHA(ctx context.Context, repoName, branch, path string) (string, error) {
	opts := &gogithub.RepositoryContentGetOptions{Ref: branch}
	file, _, resp, err := c.REST.Repositories.GetContents(ctx, c.org, repoName, path, opts)
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

// CommitFile creates or updates a single file on the given branch.
// If blobSHA is non-empty the file is treated as an update; otherwise as a create.
func (c *Client) CommitFile(ctx context.Context, repoName, branch, path, message string, content []byte, blobSHA string) error {
	opts := &gogithub.RepositoryContentFileOptions{
		Message: &message,
		Content: content,
		Branch:  &branch,
	}
	if blobSHA != "" {
		opts.SHA = &blobSHA
	}
	_, _, err := c.REST.Repositories.CreateFile(ctx, c.org, repoName, path, opts)
	if err != nil {
		return fmt.Errorf("committing file %s: %w", path, err)
	}
	return nil
}

// CreatePullRequest opens a pull request and returns its HTML URL.
func (c *Client) CreatePullRequest(ctx context.Context, repoName, title, body, head, base string) (string, error) {
	pr, _, err := c.REST.PullRequests.Create(ctx, c.org, repoName, &gogithub.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return "", fmt.Errorf("creating pull request: %w", err)
	}
	return pr.GetHTMLURL(), nil
}

// ProposeFileChange creates a branch, commits a single file change, and opens a pull request.
// Returns the PR's HTML URL. This is the standard workflow for governance rules that apply via PR.
func (c *Client) ProposeFileChange(ctx context.Context, repoName, baseBranch, filePath, branchPrefix, commitMsg, prTitle, prBody string, content []byte) (string, error) {
	baseSHA, err := c.GetBranchSHA(ctx, repoName, baseBranch)
	if err != nil {
		return "", fmt.Errorf("getting base branch SHA: %w", err)
	}

	prBranch := fmt.Sprintf("%s-%d", branchPrefix, time.Now().Unix())
	if err := c.CreateBranch(ctx, repoName, prBranch, baseSHA); err != nil {
		return "", fmt.Errorf("creating branch: %w", err)
	}

	fileSHA, err := c.GetFileSHA(ctx, repoName, baseBranch, filePath)
	if err != nil {
		return "", fmt.Errorf("getting existing file SHA: %w", err)
	}

	if err := c.CommitFile(ctx, repoName, prBranch, filePath, commitMsg, content, fileSHA); err != nil {
		return "", fmt.Errorf("committing file: %w", err)
	}

	return c.CreatePullRequest(ctx, repoName, prTitle, prBody, prBranch, baseBranch)
}
