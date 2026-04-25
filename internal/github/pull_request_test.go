package github

import (
	"fmt"
	"net/http"
	"testing"
)

func TestCreateFileChangePR(t *testing.T) {
	t.Run("success with new branch and new PR", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"commit-sha-444"}`)
		})
		mux.HandleFunc("PATCH /repos/test-org/test-repo-1/git/refs/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"commit-sha-444"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[]`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"html_url":"https://github.com/test-org/test-repo-1/pull/1"}`)
		})

		url, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "fix: update CODEOWNERS", "body", []FileChange{
			{Path: ".github/CODEOWNERS", Content: []byte("* @platform\n")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://github.com/test-org/test-repo-1/pull/1" {
			t.Errorf("url = %q, want PR URL", url)
		}
	})

	t.Run("branch already exists", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"message":"Reference already exists"}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"old-sha"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"commit-sha-444"}`)
		})
		mux.HandleFunc("PATCH /repos/test-org/test-repo-1/git/refs/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"commit-sha-444"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[]`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"html_url":"https://github.com/test-org/test-repo-1/pull/2"}`)
		})

		url, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "fix: update CODEOWNERS", "body", []FileChange{
			{Path: ".github/CODEOWNERS", Content: []byte("* @platform\n")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://github.com/test-org/test-repo-1/pull/2" {
			t.Errorf("url = %q, want PR URL", url)
		}
	})

	t.Run("existing PR is reused", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"commit-sha-444"}`)
		})
		mux.HandleFunc("PATCH /repos/test-org/test-repo-1/git/refs/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"commit-sha-444"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"html_url":"https://github.com/test-org/test-repo-1/pull/5","head":{"ref":"governance/fix"},"base":{"ref":"main"}}]`)
		})

		url, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "fix: update CODEOWNERS", "body", []FileChange{
			{Path: ".github/CODEOWNERS", Content: []byte("* @platform\n")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://github.com/test-org/test-repo-1/pull/5" {
			t.Errorf("url = %q, want existing PR URL", url)
		}
	})

	t.Run("base branch not found", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when base branch not found")
		}
	})

	t.Run("blob creation fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when blob creation fails")
		}
	})

	t.Run("force update ref fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"commit-sha-444"}`)
		})
		mux.HandleFunc("PATCH /repos/test-org/test-repo-1/git/refs/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when ref update fails")
		}
	})

	t.Run("multiple file changes", func(t *testing.T) {
		client, mux := setupTestClient(t)

		blobCount := 0
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			blobCount++
			fmt.Fprintf(w, `{"sha":"blob-sha-%d"}`, blobCount)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"commit-sha-444"}`)
		})
		mux.HandleFunc("PATCH /repos/test-org/test-repo-1/git/refs/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"commit-sha-444"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[]`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"html_url":"https://github.com/test-org/test-repo-1/pull/3"}`)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: ".github/CODEOWNERS", Content: []byte("* @platform\n")},
			{Path: "config.yaml", Content: []byte("key: value\n")},
			{Path: "README.md", Content: []byte("# hello\n")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if blobCount != 3 {
			t.Errorf("blob creations = %d, want 3", blobCount)
		}
	})

	t.Run("branch create and get both fail", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"message":"Reference already exists"}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when branch create and verify both fail")
		}
	})

	t.Run("get commit fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when GetCommit fails")
		}
	})

	t.Run("create tree fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when CreateTree fails")
		}
	})

	t.Run("create commit fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when CreateCommit fails")
		}
	})

	t.Run("list PRs fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		registerCommitHandlers(t, mux)
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when listing PRs fails")
		}
	})

	t.Run("create PR fails", func(t *testing.T) {
		client, mux := setupTestClient(t)

		registerCommitHandlers(t, mux)
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[]`)
		})
		mux.HandleFunc("POST /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"message":"Validation Failed"}`)
		})

		_, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err == nil {
			t.Fatal("expected error when PR creation fails")
		}
	})

	t.Run("existing PR found on second page", func(t *testing.T) {
		client, mux := setupTestClient(t)

		registerCommitHandlers(t, mux)
		mux.HandleFunc("GET /repos/test-org/test-repo-1/pulls", func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
				fmt.Fprint(w, `[{"html_url":"https://github.com/test-org/test-repo-1/pull/10","head":{"ref":"other-branch"},"base":{"ref":"main"}}]`)
			} else {
				fmt.Fprint(w, `[{"html_url":"https://github.com/test-org/test-repo-1/pull/11","head":{"ref":"governance/fix"},"base":{"ref":"main"}}]`)
			}
		})

		url, err := client.CreateFileChangePR(t.Context(), "test-repo-1", "main", "governance/fix", "title", "body", []FileChange{
			{Path: "f.txt", Content: []byte("x")},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://github.com/test-org/test-repo-1/pull/11" {
			t.Errorf("url = %q, want PR #11 URL", url)
		}
	})
}

// registerCommitHandlers registers the full set of handlers needed to get
// through ensureBranch + createCommitForChanges + forceUpdateRef successfully.
func registerCommitHandlers(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	mux.HandleFunc("GET /repos/test-org/test-repo-1/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/heads/main","object":{"sha":"base-sha-111"}}`)
	})
	mux.HandleFunc("POST /repos/test-org/test-repo-1/git/refs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"base-sha-111"}}`)
	})
	mux.HandleFunc("GET /repos/test-org/test-repo-1/git/commits/base-sha-111", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"sha":"base-sha-111","tree":{"sha":"tree-sha-000"}}`)
	})
	mux.HandleFunc("POST /repos/test-org/test-repo-1/git/blobs", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"sha":"blob-sha-222"}`)
	})
	mux.HandleFunc("POST /repos/test-org/test-repo-1/git/trees", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"sha":"tree-sha-333"}`)
	})
	mux.HandleFunc("POST /repos/test-org/test-repo-1/git/commits", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"sha":"commit-sha-444"}`)
	})
	mux.HandleFunc("PATCH /repos/test-org/test-repo-1/git/refs/heads/governance/fix", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ref":"refs/heads/governance/fix","object":{"sha":"commit-sha-444"}}`)
	})
}
