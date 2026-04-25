package reporter

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mkhusro-usm/myapp/rule"
)

func TestBuildReport(t *testing.T) {
	t.Run("aggregates results correctly", func(t *testing.T) {
		orgResults := []*rule.Result{
			{RuleName: "org-rule", Repository: "org-wide", Compliant: true},
		}
		repoResults := []*rule.Result{
			{RuleName: "rulesets", Repository: "repo-a", Compliant: true, Applied: true, PullRequestURL: "https://github.com/org/repo-a/pull/1"},
			{RuleName: "rulesets", Repository: "repo-b", Compliant: false, ViolationCount: 2},
			{RuleName: "codeowners", Repository: "repo-a", Compliant: false, ViolationCount: 1},
		}

		report := BuildReport("test-org", rule.ModeApply, orgResults, repoResults)

		if report.Organization != "test-org" {
			t.Errorf("Organization = %q, want %q", report.Organization, "test-org")
		}
		if report.Mode != rule.ModeApply {
			t.Errorf("Mode = %q, want %q", report.Mode, rule.ModeApply)
		}
		if report.Summary.TotalEvaluations != 4 {
			t.Errorf("TotalEvaluations = %d, want 4", report.Summary.TotalEvaluations)
		}
		if report.Summary.Repositories != 3 {
			t.Errorf("Repositories = %d, want 3", report.Summary.Repositories)
		}
		if report.Summary.CompliantResults != 2 {
			t.Errorf("CompliantResults = %d, want 2", report.Summary.CompliantResults)
		}
		if report.Summary.NonCompliantResults != 2 {
			t.Errorf("NonCompliantResults = %d, want 2", report.Summary.NonCompliantResults)
		}
		if report.Summary.AppliedResults != 1 {
			t.Errorf("AppliedResults = %d, want 1", report.Summary.AppliedResults)
		}
		if len(report.Summary.PullRequests) != 1 {
			t.Fatalf("PullRequests count = %d, want 1", len(report.Summary.PullRequests))
		}
		if report.Summary.PullRequests[0] != "https://github.com/org/repo-a/pull/1" {
			t.Errorf("PullRequests[0] = %q, want PR URL", report.Summary.PullRequests[0])
		}
		if len(report.OrgResults) != 1 {
			t.Errorf("OrgResults count = %d, want 1", len(report.OrgResults))
		}
		if len(report.RepoResults) != 3 {
			t.Errorf("RepoResults count = %d, want 3", len(report.RepoResults))
		}
	})

	t.Run("empty results", func(t *testing.T) {
		report := BuildReport("test-org", rule.ModeEvaluate, nil, nil)

		if report.Summary.TotalEvaluations != 0 {
			t.Errorf("TotalEvaluations = %d, want 0", report.Summary.TotalEvaluations)
		}
		if report.Summary.Repositories != 0 {
			t.Errorf("Repositories = %d, want 0", report.Summary.Repositories)
		}
		if report.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	})
}

func TestReportWrite(t *testing.T) {
	t.Run("write to file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "output", "report.json")

		report := BuildReport("test-org", rule.ModeEvaluate, nil, []*rule.Result{
			{RuleName: "rulesets", Repository: "repo-a", Compliant: true},
		})

		if err := report.Write(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading report file: %v", err)
		}

		var decoded Report
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshalling report: %v", err)
		}
		if decoded.Organization != "test-org" {
			t.Errorf("Organization = %q, want %q", decoded.Organization, "test-org")
		}
		if decoded.Summary.TotalEvaluations != 1 {
			t.Errorf("TotalEvaluations = %d, want 1", decoded.Summary.TotalEvaluations)
		}
	})

	t.Run("write to stdout when path is empty", func(t *testing.T) {
		report := BuildReport("test-org", rule.ModeEvaluate, nil, nil)

		origStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := report.Write("")

		w.Close()
		os.Stdout = origStdout

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var buf bytes.Buffer
		buf.ReadFrom(r)

		var decoded Report
		if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Fatalf("unmarshalling stdout output: %v", err)
		}
		if decoded.Organization != "test-org" {
			t.Errorf("Organization = %q, want %q", decoded.Organization, "test-org")
		}
	})

	t.Run("mkdir fails", func(t *testing.T) {
		report := BuildReport("test-org", rule.ModeEvaluate, nil, nil)

		err := report.Write("/dev/null/impossible/report.json")
		if err == nil {
			t.Fatal("expected error for invalid directory path")
		}
	})

	t.Run("file create fails", func(t *testing.T) {
		dir := t.TempDir()
		readOnly := filepath.Join(dir, "locked")
		os.Mkdir(readOnly, 0o555)

		report := BuildReport("test-org", rule.ModeEvaluate, nil, nil)

		err := report.Write(filepath.Join(readOnly, "report.json"))
		if err == nil {
			t.Fatal("expected error when file creation is denied")
		}
	})
}

func TestWriteJSON(t *testing.T) {
	report := BuildReport("test-org", rule.ModeEvaluate, nil, []*rule.Result{
		{RuleName: "rulesets", Repository: "repo-a", Compliant: true},
	})

	var buf bytes.Buffer
	if err := writeJSON(&buf, report); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshalling JSON: %v", err)
	}
	if decoded.Summary.CompliantResults != 1 {
		t.Errorf("CompliantResults = %d, want 1", decoded.Summary.CompliantResults)
	}
}
