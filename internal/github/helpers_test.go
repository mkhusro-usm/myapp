package github

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gogithub "github.com/google/go-github/v84/github"
)

// setupTestClient creates a test HTTP server and a Client pointed at it.
// The returned mux is used to register endpoint handlers in each test.
// The server is automatically closed when the test completes.
func setupTestClient(t *testing.T) (*Client, *http.ServeMux) {
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := NewClient(server.Client(), "test-org")
	client.WithBaseURL(server.URL+"/", server.URL+"/graphql")

	return client, mux
}

func rulesetFixture(name string) gogithub.RepositoryRuleset {
	return gogithub.RepositoryRuleset{
		Name:        name,
		Enforcement: "active",
	}
}
