package github

import (
	"fmt"
	"net/http"
	"testing"
)

func TestGetFileContent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"repository":{"object":{"text":"* @platform-team\n/docs @docs-team\n"}}}}`)
		})

		content, err := client.GetFileContent(t.Context(), "my-service", "main", ".github/CODEOWNERS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "* @platform-team\n/docs @docs-team\n" {
			t.Errorf("content = %q, want CODEOWNERS content", content)
		}
	})

	t.Run("file not found returns empty string", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"data":{"repository":{"object":null}}}`)
		})

		content, err := client.GetFileContent(t.Context(), "my-service", "main", "no-such-file")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" {
			t.Errorf("content = %q, want empty string", content)
		}
	})

	t.Run("graphql error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"errors":[{"message":"repo not found"}]}`)
		})

		_, err := client.GetFileContent(t.Context(), "nonexistent", "main", "README.md")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
