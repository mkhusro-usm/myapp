package rule

import (
	"context"
	"fmt"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

type BranchProtectionSettings struct {
	RequirePullRequestReviews    bool `yaml:"require_pull_request_reviews"`
	RequiredApprovingReviewCount int  `yaml:"required_approving_review_count"`
	RequireStatusChecks          bool `yaml:"require_status_checks"`
	RequireUpToDateBranch        bool `yaml:"require_up_to_date_branch"`
	EnforceAdmins                bool `yaml:"enforce_admins"`
	RequireLinearHistory         bool `yaml:"require_linear_history"`
	AllowForcePushes             bool `yaml:"allow_force_pushes"`
	AllowDeletions               bool `yaml:"allow_deletions"`
}

type BranchProtection struct {
	client   *gh.Client
	settings BranchProtectionSettings
}

func NewBranchProtection(client *gh.Client, settings BranchProtectionSettings) *BranchProtection {
	return &BranchProtection{
		client:   client,
		settings: settings,
	}
}

func (bp *BranchProtection) Name() string {
	return "branch_protection"
}

func (bp *BranchProtection) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	_, result, err := bp.evaluate(ctx, repo)
	return result, err
}

func (bp *BranchProtection) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	protection, result, err := bp.evaluate(ctx, repo)
	if err != nil {
		return nil, err
	}
	if result.Compliant {
		return result, nil
	}

	branch := defaultBranch(repo)
	input := bp.desiredInput()

	if protection == nil {
		_, err = bp.client.CreateBranchProtectionRule(ctx, repo.ID, branch, input)
	} else {
		err = bp.client.UpdateBranchProtectionRule(ctx, protection.ID, input)
	}
	if err != nil {
		return nil, fmt.Errorf("applying branch protection for %s: %w", repo.FullName(), err)
	}

	result.Applied = true
	return result, nil
}

// evaluate fetches branch protection rules and checks compliance.
// Returns the matching protection rule (nil if none) and the result.
func (bp *BranchProtection) evaluate(ctx context.Context, repo *gh.Repository) (*gh.BranchProtectionRule, *Result, error) {
	branch := defaultBranch(repo)

	rules, err := bp.client.GetBranchProtectionRules(ctx, repo.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching branch protection for %s: %w", repo.FullName(), err)
	}

	protection := gh.FindMatchingProtectionRule(rules, branch)
	if protection == nil {
		return nil, &Result{
			RuleName:   bp.Name(),
			Repository: repo.FullName(),
			Compliant:  false,
			Violations: []Violation{{
				Field:    "branch_protection",
				Expected: "enabled",
				Actual:   "none",
				Message:  fmt.Sprintf("no branch protection rule matches %s", branch),
			}},
		}, nil
	}

	violations := bp.check(protection)
	return protection, &Result{
		RuleName:   bp.Name(),
		Repository: repo.FullName(),
		Compliant:  len(violations) == 0,
		Violations: violations,
	}, nil
}

// desiredInput maps config settings to the API input for create/update.
func (bp *BranchProtection) desiredInput() gh.BranchProtectionInput {
	return gh.BranchProtectionInput{
		RequiresApprovingReviews:     bp.settings.RequirePullRequestReviews,
		RequiredApprovingReviewCount: bp.settings.RequiredApprovingReviewCount,
		RequiresStatusChecks:         bp.settings.RequireStatusChecks,
		RequiresStrictStatusChecks:   bp.settings.RequireUpToDateBranch,
		IsAdminEnforced:              bp.settings.EnforceAdmins,
		RequiresLinearHistory:        bp.settings.RequireLinearHistory,
		AllowsForcePushes:            bp.settings.AllowForcePushes,
		AllowsDeletions:              bp.settings.AllowDeletions,
	}
}

func (bp *BranchProtection) check(p *gh.BranchProtectionRule) []Violation {
	var violations []Violation

	if bp.settings.RequirePullRequestReviews {
		if !p.RequiresApprovingReviews {
			violations = append(violations, Violation{
				Field:    "require_pull_request_reviews",
				Expected: "true",
				Actual:   "false",
				Message:  "pull request reviews not required",
			})
		} else if p.RequiredApprovingReviewCount < bp.settings.RequiredApprovingReviewCount {
			violations = append(violations, Violation{
				Field:    "required_approving_review_count",
				Expected: fmt.Sprintf(">= %d", bp.settings.RequiredApprovingReviewCount),
				Actual:   fmt.Sprintf("%d", p.RequiredApprovingReviewCount),
				Message:  "insufficient required approving reviewers",
			})
		}
	}

	if bp.settings.EnforceAdmins && !p.IsAdminEnforced {
		violations = append(violations, Violation{
			Field:    "enforce_admins",
			Expected: "true",
			Actual:   "false",
			Message:  "admin enforcement not enabled",
		})
	}

	if bp.settings.RequireStatusChecks {
		if !p.RequiresStatusChecks {
			violations = append(violations, Violation{
				Field:    "require_status_checks",
				Expected: "true",
				Actual:   "false",
				Message:  "status checks not required",
			})
		} else if bp.settings.RequireUpToDateBranch && !p.RequiresStrictStatusChecks {
			violations = append(violations, Violation{
				Field:    "require_up_to_date_branch",
				Expected: "true",
				Actual:   "false",
				Message:  "branch not required to be up to date before merging",
			})
		}
	}

	if bp.settings.RequireLinearHistory && !p.RequiresLinearHistory {
		violations = append(violations, Violation{
			Field:    "require_linear_history",
			Expected: "true",
			Actual:   "false",
			Message:  "linear history not required",
		})
	}

	if !bp.settings.AllowForcePushes && p.AllowsForcePushes {
		violations = append(violations, Violation{
			Field:    "allow_force_pushes",
			Expected: "false",
			Actual:   "true",
			Message:  "force pushes are allowed",
		})
	}

	if !bp.settings.AllowDeletions && p.AllowsDeletions {
		violations = append(violations, Violation{
			Field:    "allow_deletions",
			Expected: "false",
			Actual:   "true",
			Message:  "branch deletions are allowed",
		})
	}

	return violations
}

func defaultBranch(repo *gh.Repository) string {
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}
	return "main"
}
