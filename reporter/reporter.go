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

// Report is the structured output of a governance run.
type Report struct {
	Timestamp    time.Time      `json:"timestamp"`
	Organization string         `json:"organization"`
	Mode         rule.Mode      `json:"mode"`
	Summary      Summary        `json:"summary"`
	Results      []*rule.Result `json:"results"`
}

// Summary holds aggregate counts for the report.
type Summary struct {
	Total        int `json:"total"`
	Compliant    int `json:"compliant"`
	NonCompliant int `json:"non-compliant"`
	Applied      int `json:"applied"`
}

// BuildReport constructs a Report from raw results and metadata.
func BuildReport(org string, mode rule.Mode, results []*rule.Result) *Report {
	var s Summary
	s.Total = len(results)
	for _, r := range results {
		if r.Applied {
			s.Applied++
		}
		if r.Compliant {
			s.Compliant++
		} else {
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

func writeJSON(w io.Writer, report *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
