package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestFullName(t *testing.T) {
	r := Repository{Owner: "acme", Name: "widgets"}
	if got := r.FullName(); got != "acme/widgets" {
		t.Errorf("FullName() = %q, want %q", got, "acme/widgets")
	}
}

func TestGetRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"repository":{
				"id": "R_abc123",
				"name": "my-service",
				"defaultBranchRef": {"name": "main"},
				"isArchived": false,
				"isFork": false
			}}}`)
		})

		repo, err := client.GetRepository(t.Context(), "my-service")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.ID != "R_abc123" {
			t.Errorf("ID = %q, want %q", repo.ID, "R_abc123")
		}
		if repo.Name != "my-service" {
			t.Errorf("Name = %q, want %q", repo.Name, "my-service")
		}
		if repo.Owner != "test-org" {
			t.Errorf("Owner = %q, want %q", repo.Owner, "test-org")
		}
		if repo.DefaultBranch != "main" {
			t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
		}
		if repo.IsArchived {
			t.Error("expected IsArchived = false")
		}
		if repo.IsFork {
			t.Error("expected IsFork = false")
		}
	})

	t.Run("archived fork", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"repository":{
				"id": "R_def456",
				"name": "forked-lib",
				"defaultBranchRef": {"name": "develop"},
				"isArchived": true,
				"isFork": true
			}}}`)
		})

		repo, err := client.GetRepository(t.Context(), "forked-lib")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.IsArchived {
			t.Error("expected IsArchived = true")
		}
		if !repo.IsFork {
			t.Error("expected IsFork = true")
		}
		if repo.DefaultBranch != "develop" {
			t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "develop")
		}
	})

	t.Run("graphql error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"errors":[{"message":"not found"}]}`)
		})

		_, err := client.GetRepository(t.Context(), "nonexistent")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestListRepositories(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"viewer":{"repositories":{
				"nodes": [
					{"id":"R_1","name":"repo-a","owner":{"login":"test-org"},"defaultBranchRef":{"name":"main"},"isArchived":false,"isFork":false},
					{"id":"R_2","name":"repo-b","owner":{"login":"test-org"},"defaultBranchRef":{"name":"main"},"isArchived":true,"isFork":false}
				],
				"pageInfo": {"endCursor":"","hasNextPage":false}
			}}}}`)
		})

		repos, err := client.ListRepositories(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repos) != 2 {
			t.Fatalf("repos count = %d, want 2", len(repos))
		}
		if repos[0].Name != "repo-a" {
			t.Errorf("repos[0].Name = %q, want %q", repos[0].Name, "repo-a")
		}
		if repos[1].IsArchived != true {
			t.Error("repos[1] should be archived")
		}
	})

	t.Run("pagination", func(t *testing.T) {
		client, mux := setupTestClient(t)

		callCount := 0
		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Variables map[string]interface{} `json:"variables"`
			}
			json.NewDecoder(r.Body).Decode(&body)

			callCount++
			cursor, _ := body.Variables["cursor"].(string)
			if cursor == "" {
				fmt.Fprint(w, `{"data":{"viewer":{"repositories":{
					"nodes": [{"id":"R_1","name":"repo-1","owner":{"login":"test-org"},"defaultBranchRef":{"name":"main"},"isArchived":false,"isFork":false}],
					"pageInfo": {"endCursor":"cursor_1","hasNextPage":true}
				}}}}`)
			} else {
				fmt.Fprint(w, `{"data":{"viewer":{"repositories":{
					"nodes": [{"id":"R_2","name":"repo-2","owner":{"login":"test-org"},"defaultBranchRef":{"name":"main"},"isArchived":false,"isFork":false}],
					"pageInfo": {"endCursor":"","hasNextPage":false}
				}}}}`)
			}
		})

		repos, err := client.ListRepositories(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repos) != 2 {
			t.Fatalf("repos count = %d, want 2", len(repos))
		}
		if callCount != 2 {
			t.Errorf("expected 2 GraphQL calls, got %d", callCount)
		}
		if repos[1].Name != "repo-2" {
			t.Errorf("repos[1].Name = %q, want %q", repos[1].Name, "repo-2")
		}
	})

	t.Run("empty result", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"viewer":{"repositories":{
				"nodes": [],
				"pageInfo": {"endCursor":"","hasNextPage":false}
			}}}}`)
		})

		repos, err := client.ListRepositories(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repos) != 0 {
			t.Errorf("repos count = %d, want 0", len(repos))
		}
	})

	t.Run("graphql error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"errors":[{"message":"rate limited"}]}`)
		})

		_, err := client.ListRepositories(t.Context())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "querying repositories") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "querying repositories")
		}
	})
}
