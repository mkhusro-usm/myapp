package rule

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// ActorList defines users and teams for actor-based branch protection settings.
type ActorList struct {
	Users []string `yaml:"users"`
	Teams []string `yaml:"teams"`
}

func (a ActorList) isEmpty() bool {
	return len(a.Users) == 0 && len(a.Teams) == 0
}

func (a ActorList) allNames() []string {
	var names []string
	for _, u := range a.Users {
		names = append(names, "user:"+u)
	}
	for _, t := range a.Teams {
		names = append(names, "team:"+t)
	}
	sort.Strings(names)
	return names
}

// BranchProtectionSettings holds the desired branch protection configuration from YAML.
// Pointer fields are optional — only non-nil fields are evaluated and applied.
type BranchProtectionSettings struct {
	// Pull request reviews
	RequirePullRequestReviews    *bool      `yaml:"require-pull-request-reviews"`
	RequiredApprovingReviewCount *int       `yaml:"required-approving-review-count"`
	DismissStaleReviews          *bool      `yaml:"dismiss-stale-reviews"`
	RequireCodeOwnerReviews      *bool      `yaml:"require-code-owner-reviews"`
	RequireLastPushApproval      *bool      `yaml:"require-last-push-approval"`
	BypassPullRequestActors      *ActorList `yaml:"bypass-pull-request-actors"`
	ReviewDismissalActors        *ActorList `yaml:"review-dismissal-actors"`

	// Status checks
	RequireStatusChecks         *bool    `yaml:"require-status-checks"`
	RequireUpToDateBranch       *bool    `yaml:"require-up-to-date-branch"`
	RequiredStatusCheckContexts []string `yaml:"required-status-check-contexts"`

	// Deployments
	RequireDeployments             *bool    `yaml:"require-deployments"`
	RequiredDeploymentEnvironments []string `yaml:"required-deployment-environments"`

	// Admin and merge rules
	EnforceAdmins                 *bool `yaml:"enforce-admins"`
	RequireLinearHistory          *bool `yaml:"require-linear-history"`
	RequireSignedCommits          *bool `yaml:"require-signed-commits"`
	RequireConversationResolution *bool `yaml:"require-conversation-resolution"`

	// Branch restrictions
	AllowForcePushes      *bool      `yaml:"allow-force-pushes"`
	AllowDeletions        *bool      `yaml:"allow-deletions"`
	BlockCreations        *bool      `yaml:"block-creations"`
	LockBranch            *bool      `yaml:"lock-branch"`
	AllowForkSyncing      *bool      `yaml:"allow-fork-syncing"`
	PushRestrictionActors *ActorList `yaml:"push-restriction-actors"`
}

// BranchProtection enforces branch protection rules on the default branch.
type BranchProtection struct {
	client   *gh.Client
	settings BranchProtectionSettings
}

// NewBranchProtection creates a BranchProtection rule with the given settings.
func NewBranchProtection(client *gh.Client, settings BranchProtectionSettings) *BranchProtection {
	return &BranchProtection{
		client:   client,
		settings: settings,
	}
}

func (bp *BranchProtection) Name() string {
	return "branch-protection"
}

func (bp *BranchProtection) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] evaluating repository branch protection", repo.FullName())
	branch := defaultBranch(repo)

	protection, err := bp.client.GetBranchProtectionRule(ctx, repo.Name, branch)
	if err != nil {
		return nil, fmt.Errorf("fetching branch protection for %s: %w", repo.FullName(), err)
	}

	if protection == nil {
		return NewResult(bp.Name(), repo.FullName(), []Violation{{
			Field:    "branch_protection",
			Expected: "enabled",
			Actual:   "none",
			Message:  fmt.Sprintf("no branch protection rule matches %s", branch),
		}}), nil
	}

	return NewResult(bp.Name(), repo.FullName(), bp.check(protection)), nil
}

func (bp *BranchProtection) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] applying repository branch protection", repo.FullName())
	branch := defaultBranch(repo)

	protection, err := bp.client.GetBranchProtectionRule(ctx, repo.Name, branch)
	if err != nil {
		return nil, fmt.Errorf("fetching branch protection for %s: %w", repo.FullName(), err)
	}

	input, err := bp.desiredInput(ctx)
	if err != nil {
		return nil, fmt.Errorf("building desired input for %s: %w", repo.FullName(), err)
	}

	if protection == nil {
		log.Printf("[%s] creating branch protection rule for %s", repo.FullName(), branch)
		_, err = bp.client.CreateBranchProtectionRule(ctx, repo.ID, branch, input)
	} else {
		log.Printf("[%s] updating branch protection rule for %s", repo.FullName(), branch)
		err = bp.client.UpdateBranchProtectionRule(ctx, protection.ID, input)
	}

	if err != nil {
		return nil, fmt.Errorf("applying branch protection for %s: %w", repo.FullName(), err)
	}

	r := NewResult(bp.Name(), repo.FullName(), nil)
	r.Applied = true

	return r, nil
}

func (bp *BranchProtection) desiredInput(ctx context.Context) (gh.BranchProtectionInput, error) {
	s := bp.settings
	input := gh.BranchProtectionInput{
		RequiresApprovingReviews:       s.RequirePullRequestReviews,
		RequiredApprovingReviewCount:   s.RequiredApprovingReviewCount,
		DismissesStaleReviews:          s.DismissStaleReviews,
		RequiresCodeOwnerReviews:       s.RequireCodeOwnerReviews,
		RequireLastPushApproval:        s.RequireLastPushApproval,
		RequiresStatusChecks:           s.RequireStatusChecks,
		RequiresStrictStatusChecks:     s.RequireUpToDateBranch,
		RequiredStatusCheckContexts:    s.RequiredStatusCheckContexts,
		RequiresDeployments:            s.RequireDeployments,
		RequiredDeploymentEnvironments: s.RequiredDeploymentEnvironments,
		IsAdminEnforced:                s.EnforceAdmins,
		RequiresLinearHistory:          s.RequireLinearHistory,
		RequiresCommitSignatures:       s.RequireSignedCommits,
		RequiresConversationResolution: s.RequireConversationResolution,
		AllowsForcePushes:              s.AllowForcePushes,
		AllowsDeletions:                s.AllowDeletions,
		BlocksCreations:                s.BlockCreations,
		LockBranch:                     s.LockBranch,
		LockAllowsFetchAndMerge:        s.AllowForkSyncing,
	}

	if s.PushRestrictionActors != nil {
		v := !s.PushRestrictionActors.isEmpty()
		input.RestrictsPushes = &v
	}
	if s.ReviewDismissalActors != nil {
		v := !s.ReviewDismissalActors.isEmpty()
		input.RestrictsReviewDismissals = &v
	}

	if s.BypassPullRequestActors != nil && !s.BypassPullRequestActors.isEmpty() {
		ids, err := bp.client.ResolveActorIDs(ctx, s.BypassPullRequestActors.Users, s.BypassPullRequestActors.Teams)
		if err != nil {
			return input, fmt.Errorf("resolving bypass pull request actors: %w", err)
		}
		input.BypassPullRequestActorIDs = ids
	}

	if s.ReviewDismissalActors != nil && !s.ReviewDismissalActors.isEmpty() {
		ids, err := bp.client.ResolveActorIDs(ctx, s.ReviewDismissalActors.Users, s.ReviewDismissalActors.Teams)
		if err != nil {
			return input, fmt.Errorf("resolving review dismissal actors: %w", err)
		}
		input.ReviewDismissalActorIDs = ids
	}

	if s.PushRestrictionActors != nil && !s.PushRestrictionActors.isEmpty() {
		ids, err := bp.client.ResolveActorIDs(ctx, s.PushRestrictionActors.Users, s.PushRestrictionActors.Teams)
		if err != nil {
			return input, fmt.Errorf("resolving push restriction actors: %w", err)
		}
		input.PushActorIDs = ids
	}

	return input, nil
}

func (bp *BranchProtection) check(p *gh.BranchProtectionRule) []Violation {
	s := bp.settings
	var violations []Violation

	checkBool := func(field string, expected *bool, actual bool) {
		if expected != nil && *expected != actual {
			violations = append(violations, Violation{
				Field:    field,
				Expected: fmt.Sprintf("%t", *expected),
				Actual:   fmt.Sprintf("%t", actual),
			})
		}
	}

	checkInt := func(field string, expected *int, actual int) {
		if expected != nil && actual < *expected {
			violations = append(violations, Violation{
				Field:    field,
				Expected: fmt.Sprintf(">= %d", *expected),
				Actual:   fmt.Sprintf("%d", actual),
			})
		}
	}

	// Pull request reviews
	checkBool("require-pull-request-reviews", s.RequirePullRequestReviews, p.RequiresApprovingReviews)
	if s.RequiredApprovingReviewCount != nil && p.RequiresApprovingReviews {
		checkInt("required-approving-review-count", s.RequiredApprovingReviewCount, p.RequiredApprovingReviewCount)
	}
	checkBool("dismiss-stale-reviews", s.DismissStaleReviews, p.DismissesStaleReviews)
	checkBool("require-code-owner-reviews", s.RequireCodeOwnerReviews, p.RequiresCodeOwnerReviews)
	checkBool("require-last-push-approval", s.RequireLastPushApproval, p.RequireLastPushApproval)

	// Actor-based: bypass pull request
	if s.BypassPullRequestActors != nil {
		violations = append(violations, checkActors(
			"bypass-pull-request-actors",
			*s.BypassPullRequestActors,
			p.BypassPullRequestActors,
		)...)
	}

	// Actor-based: review dismissal restrictions
	if s.ReviewDismissalActors != nil {
		violations = append(violations, checkActors(
			"review-dismissal-actors",
			*s.ReviewDismissalActors,
			p.ReviewDismissalActors,
		)...)
	}

	// Status checks
	checkBool("require-status-checks", s.RequireStatusChecks, p.RequiresStatusChecks)
	if s.RequireStatusChecks != nil && *s.RequireStatusChecks && p.RequiresStatusChecks {
		checkBool("require-up-to-date-branch", s.RequireUpToDateBranch, p.RequiresStrictStatusChecks)
		if s.RequiredStatusCheckContexts != nil {
			if missing := missingStrings(s.RequiredStatusCheckContexts, p.RequiredStatusCheckContexts); len(missing) > 0 {
				violations = append(violations, Violation{
					Field:    "required-status-check-contexts",
					Expected: strings.Join(s.RequiredStatusCheckContexts, ", "),
					Actual:   strings.Join(p.RequiredStatusCheckContexts, ", "),
					Message:  fmt.Sprintf("missing required checks: %s", strings.Join(missing, ", ")),
				})
			}
		}
	}

	// Deployments
	checkBool("require-deployments", s.RequireDeployments, p.RequiresDeployments)
	if s.RequireDeployments != nil && *s.RequireDeployments && p.RequiresDeployments {
		if s.RequiredDeploymentEnvironments != nil {
			if missing := missingStrings(s.RequiredDeploymentEnvironments, p.RequiredDeploymentEnvironments); len(missing) > 0 {
				violations = append(violations, Violation{
					Field:    "required-deployment-environments",
					Expected: strings.Join(s.RequiredDeploymentEnvironments, ", "),
					Actual:   strings.Join(p.RequiredDeploymentEnvironments, ", "),
					Message:  fmt.Sprintf("missing required environments: %s", strings.Join(missing, ", ")),
				})
			}
		}
	}

	// Admin and merge rules
	checkBool("enforce-admins", s.EnforceAdmins, p.IsAdminEnforced)
	checkBool("require-linear-history", s.RequireLinearHistory, p.RequiresLinearHistory)
	checkBool("require-signed-commits", s.RequireSignedCommits, p.RequiresCommitSignatures)
	checkBool("require-conversation-resolution", s.RequireConversationResolution, p.RequiresConversationResolution)

	// Branch restrictions
	checkBool("allow-force-pushes", s.AllowForcePushes, p.AllowsForcePushes)
	checkBool("allow-deletions", s.AllowDeletions, p.AllowsDeletions)
	checkBool("block-creations", s.BlockCreations, p.BlocksCreations)
	checkBool("lock-branch", s.LockBranch, p.LockBranch)
	checkBool("allow-fork-syncing", s.AllowForkSyncing, p.LockAllowsFetchAndMerge)

	// Actor-based: push restrictions
	if s.PushRestrictionActors != nil {
		wantRestrictPushes := !s.PushRestrictionActors.isEmpty()
		checkBool("restrict-pushes", &wantRestrictPushes, p.RestrictsPushes)
		if wantRestrictPushes && p.RestrictsPushes {
			violations = append(violations, checkActors(
				"push-restriction-actors",
				*s.PushRestrictionActors,
				p.PushAllowanceActors,
			)...)
		}
	}

	return violations
}

// checkActors compares desired actors (from config) against actual actors (from GitHub).
// Reports missing actors as violations.
func checkActors(field string, desired ActorList, actual []gh.Actor) []Violation {
	desiredNames := desired.allNames()
	if len(desiredNames) == 0 {
		return nil
	}

	actualNames := make(map[string]bool, len(actual))
	for _, a := range actual {
		key := strings.ToLower(a.Type) + ":" + a.Name
		actualNames[key] = true
	}

	var missing []string
	for _, name := range desiredNames {
		if !actualNames[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	var actualList []string
	for _, a := range actual {
		actualList = append(actualList, strings.ToLower(a.Type)+":"+a.Name)
	}
	sort.Strings(actualList)

	return []Violation{{
		Field:    field,
		Expected: strings.Join(desiredNames, ", "),
		Actual:   strings.Join(actualList, ", "),
		Message:  fmt.Sprintf("missing actors: %s", strings.Join(missing, ", ")),
	}}
}

// missingStrings returns required strings that are not present in actual.
func missingStrings(required, actual []string) []string {
	have := make(map[string]bool, len(actual))
	for _, c := range actual {
		have[c] = true
	}

	var missing []string
	for _, c := range required {
		if !have[c] {
			missing = append(missing, c)
		}
	}
	sort.Strings(missing)

	return missing
}

const defaultBranchFallback = "main"

func defaultBranch(repo *gh.Repository) string {
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}

	return defaultBranchFallback
}
