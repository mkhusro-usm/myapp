package github

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestGetRepoRulesetByName(t *testing.T) {
	t.Run("found on first page", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"id": 101, "name": "main", "enforcement": "active"}]`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets/101", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"id": 101,
				"name": "main",
				"enforcement": "active",
				"rules": [{"type": "creation"}],
				"bypass_actors": [{"actor_id": 1, "actor_type": "OrganizationAdmin", "bypass_mode": "always"}]
			}`)
		})

		rs, err := client.GetRepoRulesetByName(context.Background(), "test-repo-1", "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rs == nil {
			t.Fatal("expected ruleset, got nil")
		}
		if rs.GetID() != 101 {
			t.Errorf("id = %d, want 101", rs.GetID())
		}
		if rs.Name != "main" {
			t.Errorf("name = %q, want %q", rs.Name, "main")
		}
		if rs.Rules == nil {
			t.Error("expected rules to be populated from GetRuleset call")
		}
	})

	t.Run("not found", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"id": 200, "name": "other-ruleset", "enforcement": "active"}]`)
		})

		rs, err := client.GetRepoRulesetByName(context.Background(), "test-repo-1", "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rs != nil {
			t.Errorf("expected nil, got ruleset %q", rs.Name)
		}
	})

	t.Run("found on second page", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
				fmt.Fprint(w, `[{"id": 300, "name": "other", "enforcement": "active"}]`)
			} else {
				fmt.Fprint(w, `[{"id": 301, "name": "target", "enforcement": "active"}]`)
			}
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets/301", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id": 301, "name": "target", "enforcement": "active", "rules": []}`)
		})

		rs, err := client.GetRepoRulesetByName(context.Background(), "test-repo-1", "target")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rs == nil {
			t.Fatal("expected ruleset, got nil")
		}
		if rs.GetID() != 301 {
			t.Errorf("id = %d, want 301", rs.GetID())
		}
	})

	t.Run("empty list", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[]`)
		})

		rs, err := client.GetRepoRulesetByName(context.Background(), "test-repo-1", "anything")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rs != nil {
			t.Errorf("expected nil, got ruleset %q", rs.Name)
		}
	})

	t.Run("list error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, err := client.GetRepoRulesetByName(context.Background(), "test-repo-1", "main")
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})

	t.Run("get detail error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `[{"id": 400, "name": "main", "enforcement": "active"}]`)
		})
		mux.HandleFunc("GET /repos/test-org/test-repo-1/rulesets/400", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.GetRepoRulesetByName(context.Background(), "test-repo-1", "main")
		if err == nil {
			t.Fatal("expected error for 404 on GetRuleset")
		}
	})
}

func TestCreateRepoRuleset(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id": 500, "name": "main", "enforcement": "active"}`)
		})

		rs, err := client.CreateRepoRuleset(context.Background(), "test-repo-1", rulesetFixture("main"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rs.GetID() != 500 {
			t.Errorf("id = %d, want 500", rs.GetID())
		}
	})

	t.Run("api error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("POST /repos/test-org/test-repo-1/rulesets", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"message": "Validation Failed"}`)
		})

		_, err := client.CreateRepoRuleset(context.Background(), "test-repo-1", rulesetFixture("main"))
		if err == nil {
			t.Fatal("expected error for 422 response")
		}
	})
}

func TestUpdateRepoRuleset(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("PUT /repos/test-org/test-repo-1/rulesets/600", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id": 600, "name": "main", "enforcement": "active"}`)
		})

		rs, err := client.UpdateRepoRuleset(context.Background(), "test-repo-1", 600, rulesetFixture("main"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rs.GetID() != 600 {
			t.Errorf("id = %d, want 600", rs.GetID())
		}
	})

	t.Run("api error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("PUT /repos/test-org/test-repo-1/rulesets/600", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.UpdateRepoRuleset(context.Background(), "test-repo-1", 600, rulesetFixture("main"))
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})
}
