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
	"log"
	"net/http"
	"net/url"

	gogithub "github.com/google/go-github/v84/github"
	"github.com/shurcooL/githubv4"
)

const rateLimitWarnThreshold = 100

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

// WithBaseURL configures custom base URLs for both REST and GraphQL clients.
// This is primarily used for testing with httptest servers and GitHub Enterprise.
// The restURL should include a trailing slash (e.g., "http://localhost:1234/").
// The graphqlURL is the full GraphQL endpoint (e.g., "http://localhost:1234/graphql").
func (c *Client) WithBaseURL(restURL, graphqlURL string) *Client {
	u, err := url.Parse(restURL)
	if err == nil {
		c.restClient.BaseURL = u
	}
	c.graphQL = githubv4.NewEnterpriseClient(graphqlURL, c.restClient.Client())
	return c
}

// Org returns the organization name this client is configured for.
func (c *Client) Org() string {
	return c.org
}

// logRateLimit logs a warning when the remaining API rate limit falls below
// the warning threshold. Called after REST API calls to provide visibility
// into quota consumption.
func logRateLimit(resp *gogithub.Response) {
	if resp == nil {
		return
	}
	remaining := resp.Rate.Remaining
	if remaining < rateLimitWarnThreshold {
		log.Printf("warning: GitHub API rate limit low: %d/%d remaining (resets at %s)",
			remaining, resp.Rate.Limit, resp.Rate.Reset.Time.Format("15:04:05"))
	}
}
