package reporter

import (
	"context"
	"time"

	"github.com/mkhusro-usm/myapp/rule"
)

// Report is the top-level structure written by reporters.
type Report struct {
	Timestamp    time.Time      `json:"timestamp"`
	Organization string         `json:"organization"`
	Mode         string         `json:"mode"`
	Summary      Summary        `json:"summary"`
	Results      []*rule.Result `json:"results"`
}

// Summary holds aggregate counts for the report.
type Summary struct {
	Total        int `json:"total"`
	Compliant    int `json:"compliant"`
	NonCompliant int `json:"non_compliant"`
	Applied      int `json:"applied"`
}

// Reporter defines how governance results are output.
type Reporter interface {
	Report(ctx context.Context, report *Report) error
}

// BuildReport constructs a Report from raw results and metadata.
func BuildReport(org, mode string, results []*rule.Result) *Report {
	var s Summary
	s.Total = len(results)
	for _, r := range results {
		switch {
		case r.Compliant:
			s.Compliant++
		case r.Applied:
			s.Applied++
		default:
			s.NonCompliant++
		}
	}
	return &Report{
		Timestamp:    time.Now().UTC(),
		Organization: org,
		Mode:         mode,
		Summary:      s,
		Results:      results,
	}
}
