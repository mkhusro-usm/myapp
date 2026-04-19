package github

import (
	"net/http"

	gogithub "github.com/google/go-github/v84/github"
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
