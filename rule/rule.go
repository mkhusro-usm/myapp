// Package rule provides governance rules and core types for evaluating
// and applying repository policies.
//
// Governance rules implement RepoRule or OrgRule interfaces to evaluate compliance
// and optionally apply fixes via pull requests.
//
// Key files:
//   - rule.go: Core types (Result, Violation) and interfaces (RepoRule, OrgRule)
//   - registry.go: Registry for managing enabled rules
//   - mode.go: Run modes (evaluate, apply)
//   - codeowners.go: CODEOWNERS file enforcement
//   - rulesets.go: GitHub Rulesets config types and build/apply logic
//   - rulesets_checker.go: Ruleset drift checker (violation detection)
//   - repo_settings.go: Repository settings enforcement
package rule

import (
	"context"

	gh "github.com/mkhusro-usm/myapp/internal/github"
	"gopkg.in/yaml.v3"
)

// Result holds the outcome of evaluating or applying a single rule against a single repository.
type Result struct {
	RuleName       string      `json:"rule_name"`
	Repository     string      `json:"repository"`
	Compliant      bool        `json:"compliant"`
	ViolationCount int         `json:"violation_count"`
	Violations     []Violation `json:"violations,omitempty"`
	Applied        bool        `json:"applied"`
	PullRequestURL string      `json:"pull_request_url,omitempty"`
}

// Violation describes a specific policy drift found during evaluation.
type Violation struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

// RepoRule is a governance rule that operates on a single repository.
type RepoRule interface {
	Name() string
	Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error)
	Apply(ctx context.Context, repo *gh.Repository) (*Result, error)
}

// OrgRule is a governance rule that operates at the organization level.
type OrgRule interface {
	Name() string
	Evaluate(ctx context.Context) (*Result, error)
	Apply(ctx context.Context) (*Result, error)
}

// NewResult constructs a Result, automatically setting ViolationCount and Compliant.
func NewResult(ruleName, repository string, violations []Violation) *Result {
	violationCount := len(violations)

	return &Result{
		RuleName:       ruleName,
		Repository:     repository,
		Compliant:      violationCount == 0,
		ViolationCount: violationCount,
		Violations:     violations,
	}
}

// ParseSettings converts a generic settings map into a typed struct via YAML round-trip.
// Each rule defines its own settings struct with yaml tags.
func ParseSettings[T any](raw map[string]any) (T, error) {
	var settings T
	bytes, err := yaml.Marshal(raw)
	if err != nil {
		return settings, err
	}

	err = yaml.Unmarshal(bytes, &settings)

	return settings, err
}

const defaultBranchFallback = "main"

// defaultBranch returns the repository's default branch, falling back to "main".
func defaultBranch(repo *gh.Repository) string {
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}
	return defaultBranchFallback
}
