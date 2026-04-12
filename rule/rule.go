package rule

import (
	"context"

	gh "github.com/mkhusro-usm/myapp/internal/github"
	"gopkg.in/yaml.v3"
)

// Result holds the outcome of evaluating or applying a single rule against a single repository.
type Result struct {
	RuleName   string      `json:"rule_name"`
	Repository string      `json:"repository"`
	Compliant  bool        `json:"compliant"`
	Violations []Violation `json:"violations,omitempty"`
	Applied    bool        `json:"applied"`
}

// Violation describes a specific policy drift found during evaluation.
type Violation struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

// Rule is the interface that all governance rules must implement.
type Rule interface {
	Name() string
	Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error)
	Apply(ctx context.Context, repo *gh.Repository) (*Result, error)
}

// ParseSettings converts a generic settings map into a typed struct via YAML round-trip.
// Each rule defines its own settings struct with yaml tags.
func ParseSettings[T any](raw map[string]interface{}) (T, error) {
	var settings T
	bytes, err := yaml.Marshal(raw)
	if err != nil {
		return settings, err
	}
	err = yaml.Unmarshal(bytes, &settings)
	return settings, err
}
