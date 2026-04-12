package reporter

import (
	"context"
	"log"
)

// Console writes human-readable output to stdout.
type Console struct{}

func (c *Console) Report(_ context.Context, report *Report) error {
	for _, r := range report.Results {
		switch {
		case r.Compliant:
			log.Printf("[PASS] %s - %s", r.Repository, r.RuleName)
		case r.Applied:
			log.Printf("[APPLIED] %s - %s", r.Repository, r.RuleName)
			for _, v := range r.Violations {
				log.Printf("  -> fixed: %s (was: %s, now: %s)", v.Message, v.Actual, v.Expected)
			}
		default:
			log.Printf("[FAIL] %s - %s", r.Repository, r.RuleName)
			for _, v := range r.Violations {
				log.Printf("  -> %s (expected: %s, actual: %s)", v.Message, v.Expected, v.Actual)
			}
		}
	}

	log.Printf("\nSummary: %d compliant, %d non-compliant, %d applied — %d total evaluations",
		report.Summary.Compliant, report.Summary.NonCompliant, report.Summary.Applied, report.Summary.Total)
	return nil
}
