package rule

import (
	"context"
	"fmt"
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

type BranchProtectionSettings struct {
	// Pull request reviews
	RequirePullRequestReviews    bool      `yaml:"require-pull-request-reviews"`
	RequiredApprovingReviewCount int       `yaml:"required-approving-review-count"`
	DismissStaleReviews          bool      `yaml:"dismiss-stale-reviews"`
	RequireCodeOwnerReviews      bool      `yaml:"require-code-owner-reviews"`
	RequireLastPushApproval      bool      `yaml:"require-last-push-approval"`
	BypassPullRequestActors      ActorList `yaml:"bypass-pull-request-actors"`
	ReviewDismissalActors        ActorList `yaml:"review-dismissal-actors"`

	// Status checks
	RequireStatusChecks         bool     `yaml:"require-status-checks"`
	RequireUpToDateBranch       bool     `yaml:"require-up-to-date-branch"`
	RequiredStatusCheckContexts []string `yaml:"required-status-check-contexts"`

	// Deployments
	RequireDeployments             bool     `yaml:"require-deployments"`
	RequiredDeploymentEnvironments []string `yaml:"required-deployment-environments"`

	// Admin and merge rules
	EnforceAdmins                 bool `yaml:"enforce-admins"`
	RequireLinearHistory          bool `yaml:"require-linear-history"`
	RequireSignedCommits          bool `yaml:"require-signed-commits"`
	RequireConversationResolution bool `yaml:"require-conversation-resolution"`

	// Branch restrictions
	AllowForcePushes    bool      `yaml:"allow-force-pushes"`
	AllowDeletions      bool      `yaml:"allow-deletions"`
	BlockCreations      bool      `yaml:"block-creations"`
	LockBranch          bool      `yaml:"lock-branch"`
	AllowForkSyncing    bool      `yaml:"allow-fork-syncing"`
	PushRestrictionActors ActorList `yaml:"push-restriction-actors"`
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
	branch := defaultBranch(repo)

	protection, err := bp.client.GetBranchProtectionRule(ctx, repo.Name, branch)
	if err != nil {
		return nil, fmt.Errorf("fetching branch protection for %s: %w", repo.FullName(), err)
	}
	if protection == nil {
		return &Result{
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
	return &Result{
		RuleName:   bp.Name(),
		Repository: repo.FullName(),
		Compliant:  len(violations) == 0,
		Violations: violations,
	}, nil
}

func (bp *BranchProtection) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
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
		_, err = bp.client.CreateBranchProtectionRule(ctx, repo.ID, branch, input)
	} else {
		err = bp.client.UpdateBranchProtectionRule(ctx, protection.ID, input)
	}
	if err != nil {
		return nil, fmt.Errorf("applying branch protection for %s: %w", repo.FullName(), err)
	}

	return &Result{
		RuleName:   bp.Name(),
		Repository: repo.FullName(),
		Compliant:  true,
		Applied:    true,
	}, nil
}

func (bp *BranchProtection) desiredInput(ctx context.Context) (gh.BranchProtectionInput, error) {
	input := gh.BranchProtectionInput{
		RequiresApprovingReviews:       bp.settings.RequirePullRequestReviews,
		RequiredApprovingReviewCount:   bp.settings.RequiredApprovingReviewCount,
		DismissesStaleReviews:          bp.settings.DismissStaleReviews,
		RequiresCodeOwnerReviews:       bp.settings.RequireCodeOwnerReviews,
		RequireLastPushApproval:        bp.settings.RequireLastPushApproval,
		RequiresStatusChecks:           bp.settings.RequireStatusChecks,
		RequiresStrictStatusChecks:     bp.settings.RequireUpToDateBranch,
		RequiredStatusCheckContexts:    bp.settings.RequiredStatusCheckContexts,
		RequiresDeployments:            bp.settings.RequireDeployments,
		RequiredDeploymentEnvironments: bp.settings.RequiredDeploymentEnvironments,
		IsAdminEnforced:                bp.settings.EnforceAdmins,
		RequiresLinearHistory:          bp.settings.RequireLinearHistory,
		RequiresCommitSignatures:       bp.settings.RequireSignedCommits,
		RequiresConversationResolution: bp.settings.RequireConversationResolution,
		AllowsForcePushes:              bp.settings.AllowForcePushes,
		AllowsDeletions:                bp.settings.AllowDeletions,
		BlocksCreations:                bp.settings.BlockCreations,
		LockBranch:                     bp.settings.LockBranch,
		LockAllowsFetchAndMerge:        bp.settings.AllowForkSyncing,
		RestrictsPushes:                !bp.settings.PushRestrictionActors.isEmpty(),
		RestrictsReviewDismissals:      !bp.settings.ReviewDismissalActors.isEmpty(),
	}

	if !bp.settings.BypassPullRequestActors.isEmpty() {
		ids, err := bp.client.ResolveActorIDs(ctx, bp.settings.BypassPullRequestActors.Users, bp.settings.BypassPullRequestActors.Teams)
		if err != nil {
			return input, fmt.Errorf("resolving bypass pull request actors: %w", err)
		}
		input.BypassPullRequestActorIDs = ids
	}

	if !bp.settings.ReviewDismissalActors.isEmpty() {
		ids, err := bp.client.ResolveActorIDs(ctx, bp.settings.ReviewDismissalActors.Users, bp.settings.ReviewDismissalActors.Teams)
		if err != nil {
			return input, fmt.Errorf("resolving review dismissal actors: %w", err)
		}
		input.ReviewDismissalActorIDs = ids
	}

	if !bp.settings.PushRestrictionActors.isEmpty() {
		ids, err := bp.client.ResolveActorIDs(ctx, bp.settings.PushRestrictionActors.Users, bp.settings.PushRestrictionActors.Teams)
		if err != nil {
			return input, fmt.Errorf("resolving push restriction actors: %w", err)
		}
		input.PushActorIDs = ids
	}

	return input, nil
}

func (bp *BranchProtection) check(p *gh.BranchProtectionRule) []Violation {
	var violations []Violation

	addBool := func(field string, expected, actual bool) {
		if expected != actual {
			violations = append(violations, Violation{
				Field:    field,
				Expected: fmt.Sprintf("%t", expected),
				Actual:   fmt.Sprintf("%t", actual),
			})
		}
	}

	// Pull request reviews
	addBool("require-pull-request-reviews", bp.settings.RequirePullRequestReviews, p.RequiresApprovingReviews)
	if bp.settings.RequirePullRequestReviews && p.RequiresApprovingReviews {
		if p.RequiredApprovingReviewCount < bp.settings.RequiredApprovingReviewCount {
			violations = append(violations, Violation{
				Field:    "required-approving-review-count",
				Expected: fmt.Sprintf(">= %d", bp.settings.RequiredApprovingReviewCount),
				Actual:   fmt.Sprintf("%d", p.RequiredApprovingReviewCount),
			})
		}
	}
	addBool("dismiss-stale-reviews", bp.settings.DismissStaleReviews, p.DismissesStaleReviews)
	addBool("require-code-owner-reviews", bp.settings.RequireCodeOwnerReviews, p.RequiresCodeOwnerReviews)
	addBool("require-last-push-approval", bp.settings.RequireLastPushApproval, p.RequireLastPushApproval)

	// Actor-based: bypass pull request
	violations = append(violations, checkActors(
		"bypass-pull-request-actors",
		bp.settings.BypassPullRequestActors,
		p.BypassPullRequestActors,
	)...)

	// Actor-based: review dismissal restrictions
	violations = append(violations, checkActors(
		"review-dismissal-actors",
		bp.settings.ReviewDismissalActors,
		p.ReviewDismissalActors,
	)...)

	// Status checks
	addBool("require-status-checks", bp.settings.RequireStatusChecks, p.RequiresStatusChecks)
	if bp.settings.RequireStatusChecks && p.RequiresStatusChecks {
		addBool("require-up-to-date-branch", bp.settings.RequireUpToDateBranch, p.RequiresStrictStatusChecks)
		if missing := missingStrings(bp.settings.RequiredStatusCheckContexts, p.RequiredStatusCheckContexts); len(missing) > 0 {
			violations = append(violations, Violation{
				Field:    "required-status-check-contexts",
				Expected: strings.Join(bp.settings.RequiredStatusCheckContexts, ", "),
				Actual:   strings.Join(p.RequiredStatusCheckContexts, ", "),
				Message:  fmt.Sprintf("missing required checks: %s", strings.Join(missing, ", ")),
			})
		}
	}

	// Deployments
	addBool("require-deployments", bp.settings.RequireDeployments, p.RequiresDeployments)
	if bp.settings.RequireDeployments && p.RequiresDeployments {
		if missing := missingStrings(bp.settings.RequiredDeploymentEnvironments, p.RequiredDeploymentEnvironments); len(missing) > 0 {
			violations = append(violations, Violation{
				Field:    "required-deployment-environments",
				Expected: strings.Join(bp.settings.RequiredDeploymentEnvironments, ", "),
				Actual:   strings.Join(p.RequiredDeploymentEnvironments, ", "),
				Message:  fmt.Sprintf("missing required environments: %s", strings.Join(missing, ", ")),
			})
		}
	}

	// Admin and merge rules
	addBool("enforce-admins", bp.settings.EnforceAdmins, p.IsAdminEnforced)
	addBool("require-linear-history", bp.settings.RequireLinearHistory, p.RequiresLinearHistory)
	addBool("require-signed-commits", bp.settings.RequireSignedCommits, p.RequiresCommitSignatures)
	addBool("require-conversation-resolution", bp.settings.RequireConversationResolution, p.RequiresConversationResolution)

	// Branch restrictions
	addBool("allow-force-pushes", bp.settings.AllowForcePushes, p.AllowsForcePushes)
	addBool("allow-deletions", bp.settings.AllowDeletions, p.AllowsDeletions)
	addBool("block-creations", bp.settings.BlockCreations, p.BlocksCreations)
	addBool("lock-branch", bp.settings.LockBranch, p.LockBranch)
	addBool("allow-fork-syncing", bp.settings.AllowForkSyncing, p.LockAllowsFetchAndMerge)

	// Actor-based: push restrictions
	wantRestrictPushes := !bp.settings.PushRestrictionActors.isEmpty()
	addBool("restrict-pushes", wantRestrictPushes, p.RestrictsPushes)
	if wantRestrictPushes && p.RestrictsPushes {
		violations = append(violations, checkActors(
			"push-restriction-actors",
			bp.settings.PushRestrictionActors,
			p.PushAllowanceActors,
		)...)
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

func defaultBranch(repo *gh.Repository) string {
	if repo.DefaultBranch != "" {
		return repo.DefaultBranch
	}
	return "main"
}
