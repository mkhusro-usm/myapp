package github

import (
	"context"
	"fmt"

	"github.com/shurcooL/githubv4"
)

// GetFileContent fetches the text content of a file from a repository via GraphQL.
// branch is the git ref (e.g., "main") and path is the file path (e.g., ".github/CODEOWNERS").
func (c *Client) GetFileContent(ctx context.Context, repoName, branch, path string) (string, error) {
	expression := branch + ":" + path

	var q struct {
		Repository struct {
			Object struct {
				Blob struct {
					Text string
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $expression)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner":      githubv4.String(c.org),
		"name":       githubv4.String(repoName),
		"expression": githubv4.String(expression),
	}

	if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
		return "", fmt.Errorf("querying file content for %s/%s at %s: %w", repoName, path, branch, err)
	}

	return q.Repository.Object.Blob.Text, nil
}
