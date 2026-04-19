// Package github provides a client for interacting with GitHub's REST and GraphQL APIs.
//
// The Client wraps both GitHub's REST and GraphQL APIs for querying repositories,
// fetching file content, managing rulesets, and updating repository settings.
//
// Key files:
//   - client.go: Client struct and initialization
//   - repositories.go: Repository listing and retrieval via GraphQL
//   - content.go: File content retrieval via GraphQL
//   - pull_request.go: Branch creation, file commits, and pull requests via REST
//   - repo_settings.go: Repository settings via REST
//   - rulesets.go: Repository rulesets via REST
package github

import (
	"net/http"

	gogithub "github.com/google/go-github/v84/github"
	"github.com/shurcooL/githubv4"
)

// Client wraps both the GitHub REST and GraphQL APIs.
// The REST and GraphQL clients are private and accessible only through methods.
// GraphQL is the primary API; REST is available as a fallback.
type Client struct {
	restClient *gogithub.Client
	graphQL    *githubv4.Client
	org        string
}

// NewClient creates a Client wrapping both REST and GraphQL APIs.
func NewClient(httpClient *http.Client, org string) *Client {
	return &Client{
		restClient: gogithub.NewClient(httpClient),
		graphQL:    githubv4.NewClient(httpClient),
		org:        org,
	}
}

// Org returns the organization name this client is configured for.
func (c *Client) Org() string {
	return c.org
}
