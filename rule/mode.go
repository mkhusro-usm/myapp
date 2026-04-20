package rule

import "fmt"

// Mode represents the run mode of the governance application.
// ModeEvaluate checks compliance without making changes.
// ModeApply creates pull requests to fix violations.
type Mode string

const (
	ModeEvaluate Mode = "evaluate"
	ModeApply    Mode = "apply"
)

// ParseMode converts a string to a validated Mode.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case ModeEvaluate, ModeApply:
		return Mode(s), nil
	default:
		return "", fmt.Errorf("invalid mode %q: must be %q or %q", s, ModeEvaluate, ModeApply)
	}
}
