package github

import (
	"net/http"

	gogithub "github.com/google/go-github/v62/github"
	"github.com/shurcooL/githubv4"
)

// Client wraps both the GitHub REST and GraphQL APIs.
// GraphQL is the primary API; REST is available as a fallback.
type Client struct {
	REST    *gogithub.Client
	GraphQL *githubv4.Client
	org     string
}

// NewClient creates a Client wrapping both REST and GraphQL APIs.
func NewClient(httpClient *http.Client, org string) *Client {
	return &Client{
		REST:    gogithub.NewClient(httpClient),
		GraphQL: githubv4.NewClient(httpClient),
		org:     org,
	}
}

func (c *Client) Org() string {
	return c.org
}

func toGitHubStrings(ss []string) []githubv4.String {
	out := make([]githubv4.String, len(ss))
	for i, s := range ss {
		out[i] = githubv4.String(s)
	}
	return out
}

func toGitHubStringsPtr(ss []string) *[]githubv4.String {
	out := toGitHubStrings(ss)
	return &out
}

func toGitHubIDsPtr(ids []string) *[]githubv4.ID {
	out := make([]githubv4.ID, len(ids))
	for i, id := range ids {
		out[i] = githubv4.ID(id)
	}
	return &out
}
