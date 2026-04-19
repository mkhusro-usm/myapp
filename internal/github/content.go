package github

import (
	"context"
	"fmt"

	"github.com/shurcooL/githubv4"
)

// GetFileContent fetches the text content of a file from a repository via GraphQL.
// The branch is a git ref (e.g., "main") and path is the file path (e.g., ".github/CODEOWNERS").
// It returns an empty string if the file does not exist.
func (c *Client) GetFileContent(ctx context.Context, repoName, branch, path string) (string, error) {
	// GitHub uses "branch:path" expression syntax to reference file at a specific ref.
	expression := branch + ":" + path

	// Query structure maps to GraphQL:
	//   query {
	//     repository(owner: $owner, name: $name) {
	//       object(expression: $expression) {
	//         ... on Blob { text }
	//       }
	//     }
	//   }
	//
	// The "repository" field fetches the repo by owner and name.
	// The "object" field returns a GitObject interface (Commit, Blob, Tree, etc.), so we use
	// an inline fragment "... on Blob" to specify we want the Blob type and its text field.
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

	if err := c.graphQL.Query(ctx, &q, variables); err != nil {
		return "", fmt.Errorf("querying file content for %s/%s at %s: %w", repoName, path, branch, err)
	}

	return q.Repository.Object.Blob.Text, nil
}
