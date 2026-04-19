// Package reporter provides types and functions for generating governance run reports.
//
// Reports are structured JSON documents containing rule evaluation results,
// summaries, and metadata about the run.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mkhusro-usm/myapp/rule"
)

// Report represents the structured output of a governance run.
// It includes timing information, organization details, a summary of results,
// and detailed results for both org-scoped and repo-scoped rules.
type Report struct {
	Timestamp    time.Time      `json:"timestamp"`
	Organization string         `json:"organization"`
	Mode         rule.Mode      `json:"mode"`
	Summary      Summary        `json:"summary"`
	OrgResults   []*rule.Result `json:"org_results,omitempty"`
	RepoResults  []*rule.Result `json:"repo_results,omitempty"`
}

// Summary holds aggregate counts for the entire governance run.
// These values are derived from individual rule results.
type Summary struct {
	Repositories        int      `json:"repositories"`
	TotalEvaluations    int      `json:"total_evaluations"`
	CompliantResults    int      `json:"compliant_results"`
	NonCompliantResults int      `json:"non_compliant_results"`
	AppliedResults      int      `json:"applied_results"`
	PullRequests        []string `json:"pull_requests,omitempty"`
}

// BuildReport constructs a Report from raw results and metadata.
// It computes summary statistics by aggregating over all results.
func BuildReport(org string, mode rule.Mode, orgResults, repoResults []*rule.Result) *Report {
	var s Summary
	repos := make(map[string]struct{})
	all := append(orgResults, repoResults...)
	s.TotalEvaluations = len(all)
	for _, r := range all {
		repos[r.Repository] = struct{}{}
		if r.Applied {
			s.AppliedResults++
		}
		if r.Compliant {
			s.CompliantResults++
		} else {
			s.NonCompliantResults++
		}
		if r.PullRequestURL != "" {
			s.PullRequests = append(s.PullRequests, r.PullRequestURL)
		}
	}
	s.Repositories = len(repos)

	return &Report{
		Timestamp:    time.Now().UTC(),
		Organization: org,
		Mode:         mode,
		Summary:      s,
		OrgResults:   orgResults,
		RepoResults:  repoResults,
	}
}

// Write writes the report as JSON. If outputPath is empty, it writes to stdout.
// Otherwise, it writes to the given file path, creating directories as needed.
func (r *Report) Write(outputPath string) error {
	if outputPath == "" {
		return writeJSON(os.Stdout, r)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating report file: %w", err)
	}
	defer f.Close()

	return writeJSON(f, r)
}

// writeJSON writes the report as indented JSON to the given writer.
func writeJSON(w io.Writer, report *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(report)
}
