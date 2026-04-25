package github

import (
	"fmt"
	"net/http"
	"testing"
)

func TestGetRepoSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{
				"allow_merge_commit": true,
				"allow_squash_merge": true,
				"allow_rebase_merge": false,
				"allow_auto_merge": false,
				"delete_branch_on_merge": true,
				"allow_update_branch": true,
				"squash_merge_commit_title": "PR_TITLE",
				"squash_merge_commit_message": "BLANK",
				"merge_commit_title": "MERGE_MESSAGE",
				"merge_commit_message": "PR_BODY"
			}`)
		})

		s, err := client.GetRepoSettings(t.Context(), "test-repo-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.AllowMergeCommit == nil || *s.AllowMergeCommit != true {
			t.Errorf("AllowMergeCommit = %v, want true", s.AllowMergeCommit)
		}
		if s.AllowRebaseMerge == nil || *s.AllowRebaseMerge != false {
			t.Errorf("AllowRebaseMerge = %v, want false", s.AllowRebaseMerge)
		}
		if s.DeleteBranchOnMerge == nil || *s.DeleteBranchOnMerge != true {
			t.Errorf("DeleteBranchOnMerge = %v, want true", s.DeleteBranchOnMerge)
		}
		if s.SquashMergeCommitTitle == nil || *s.SquashMergeCommitTitle != "PR_TITLE" {
			t.Errorf("SquashMergeCommitTitle = %v, want PR_TITLE", s.SquashMergeCommitTitle)
		}
		if s.MergeCommitMessage == nil || *s.MergeCommitMessage != "PR_BODY" {
			t.Errorf("MergeCommitMessage = %v, want PR_BODY", s.MergeCommitMessage)
		}
	})

	t.Run("api error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("GET /repos/test-org/test-repo-1", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.GetRepoSettings(t.Context(), "test-repo-1")
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})
}

func TestUpdateRepoSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("PATCH /repos/test-org/test-repo-1", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"id": 1, "name": "test-repo-1"}`)
		})

		trueVal := true
		falseVal := false
		title := "PR_TITLE"
		err := client.UpdateRepoSettings(t.Context(), "test-repo-1", &RepoSettings{
			AllowSquashMerge:       &trueVal,
			AllowRebaseMerge:       &falseVal,
			DeleteBranchOnMerge:    &trueVal,
			SquashMergeCommitTitle: &title,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("api error", func(t *testing.T) {
		client, mux := setupTestClient(t)

		mux.HandleFunc("PATCH /repos/test-org/test-repo-1", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message": "Resource not accessible by integration"}`)
		})

		trueVal := true
		err := client.UpdateRepoSettings(t.Context(), "test-repo-1", &RepoSettings{
			AllowAutoMerge: &trueVal,
		})
		if err == nil {
			t.Fatal("expected error for 403 response")
		}
	})
}
