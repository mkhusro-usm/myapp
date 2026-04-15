package github

import (
	"context"
	"fmt"

	"github.com/shurcooL/githubv4"
)

// GetFileContent fetches the text content of a file from a repository via GraphQL.
// expression should be in the format "branch:path/to/file" (e.g., "main:CODEOWNERS").
func (c *Client) GetFileContent(ctx context.Context, repoName, expression string) (string, error) {
	var q struct {
		Repository struct {
			Object struct {
				Blob struct {
					Text string
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $expression)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":      githubv4.String(c.org),
		"name":       githubv4.String(repoName),
		"expression": githubv4.String(expression),
	}

	if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
		return "", fmt.Errorf("querying file content for %s/%s: %w", repoName, expression, err)
	}

	return q.Repository.Object.Blob.Text, nil
}
