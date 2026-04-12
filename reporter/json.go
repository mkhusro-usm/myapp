package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// JSON writes the report as a JSON file to the configured path.
type JSON struct {
	OutputPath string
}

func (j *JSON) Report(_ context.Context, report *Report) error {
	if err := os.MkdirAll(filepath.Dir(j.OutputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}

	if err := os.WriteFile(j.OutputPath, data, 0o644); err != nil {
		return fmt.Errorf("writing report to %s: %w", j.OutputPath, err)
	}
	return nil
}
