package rule

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	gogithub "github.com/google/go-github/v84/github"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// ---------------------------------------------------------------------------
// Config types (parsed from YAML)
// ---------------------------------------------------------------------------

// RulesetsSettings is the top-level settings for the rulesets governance rule.
type RulesetsSettings struct {
	Rulesets []RulesetConfig `yaml:"rulesets"`
}

// RulesetConfig describes a single desired ruleset.
type RulesetConfig struct {
	Name         string              `yaml:"name"`
	Target       string              `yaml:"target"`      // "branch" or "tag"
	Enforcement  string              `yaml:"enforcement"` // "active", "evaluate", "disabled"
	BypassActors []BypassActorConfig `yaml:"bypass-actors"`
	Conditions   *ConditionsConfig   `yaml:"conditions"`
	Rules        RulesConfig         `yaml:"rules"`
}

// BypassActorConfig describes an actor allowed to bypass the ruleset.
type BypassActorConfig struct {
	ActorID    int64  `yaml:"actor-id"`    // defaults to 1 for OrganizationAdmin
	ActorType  string `yaml:"actor-type"`  // OrganizationAdmin, RepositoryRole, Team, Integration
	BypassMode string `yaml:"bypass-mode"` // always, pull_request
}

// ConditionsConfig specifies which branches and repositories a ruleset targets.
type ConditionsConfig struct {
	RefName        *RefConditionConfig  `yaml:"ref-name"`
	RepositoryName *RepoConditionConfig `yaml:"repository-name"`
}

// RefConditionConfig specifies branch/tag ref name patterns.
type RefConditionConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// RepoConditionConfig specifies repository name patterns (org-scoped rulesets only).
type RepoConditionConfig struct {
	Include   []string `yaml:"include"`
	Exclude   []string `yaml:"exclude"`
	Protected *bool    `yaml:"protected"`
}

// RulesConfig holds the individual rules within a ruleset.
// Pointer fields are optional — nil means the rule is not managed by this config.
type RulesConfig struct {
	// Parameterized rules
	PullRequest          *PullRequestRuleConfig          `yaml:"pull-request"`
	RequiredStatusChecks *RequiredStatusChecksRuleConfig `yaml:"required-status-checks"`
	RequiredDeployments  *RequiredDeploymentsRuleConfig  `yaml:"required-deployments"`
	CodeScanning         *CodeScanningRuleConfig         `yaml:"code-scanning"`

	// Copilot code review
	CopilotCodeReview *CopilotCodeReviewRuleConfig `yaml:"copilot-code-review"`

	// Toggle rules: true = enforced, false = must not be present, nil = unmanaged
	Creation              *bool `yaml:"creation"`
	Update                *bool `yaml:"update"`
	Deletion              *bool `yaml:"deletion"`
	RequiredLinearHistory *bool `yaml:"required-linear-history"`
	RequiredSignatures    *bool `yaml:"required-signatures"`
	NonFastForward        *bool `yaml:"non-fast-forward"`
}

// PullRequestRuleConfig maps to the pull_request rule parameters.
type PullRequestRuleConfig struct {
	DismissStaleReviewsOnPush      bool `yaml:"dismiss-stale-reviews-on-push"`
	RequireCodeOwnerReview         bool `yaml:"require-code-owner-review"`
	RequireLastPushApproval        bool `yaml:"require-last-push-approval"`
	RequiredApprovingReviewCount   int  `yaml:"required-approving-review-count"`
	RequiredReviewThreadResolution bool `yaml:"required-review-thread-resolution"`
}

// RequiredStatusChecksRuleConfig maps to the required_status_checks rule parameters.
type RequiredStatusChecksRuleConfig struct {
	StrictRequiredStatusChecksPolicy bool                `yaml:"strict-required-status-checks-policy"`
	RequiredStatusChecks             []StatusCheckConfig `yaml:"required-status-checks"`
}

// StatusCheckConfig describes one required status check.
type StatusCheckConfig struct {
	Context       string `yaml:"context"`
	IntegrationID *int64 `yaml:"integration-id"`
}

// RequiredDeploymentsRuleConfig maps to the required_deployments rule parameters.
type RequiredDeploymentsRuleConfig struct {
	RequiredDeploymentEnvironments []string `yaml:"required-deployment-environments"`
}

// CodeScanningRuleConfig maps to the code_scanning rule parameters.
type CodeScanningRuleConfig struct {
	CodeScanningTools []CodeScanningToolConfig `yaml:"code-scanning-tools"`
}

// CodeScanningToolConfig describes one required code scanning tool.
type CodeScanningToolConfig struct {
	Tool                    string `yaml:"tool"`
	AlertsThreshold         string `yaml:"alerts-threshold"`          // none, errors, errors_and_warnings, all
	SecurityAlertsThreshold string `yaml:"security-alerts-threshold"` // none, critical, high_or_higher, medium_or_higher, all
}

// CopilotCodeReviewRuleConfig maps to the copilot_code_review rule parameters.
type CopilotCodeReviewRuleConfig struct {
	ReviewOnPush            bool `yaml:"review-on-push"`
	ReviewDraftPullRequests bool `yaml:"review-draft-pull-requests"`
}

// ---------------------------------------------------------------------------
// RepoRulesets — implements RepoRule
// ---------------------------------------------------------------------------

// RepoRulesets enforces rulesets at the repository level.
type RepoRulesets struct {
	client   *gh.Client
	settings RulesetsSettings
}

// NewRepoRulesets creates a RepoRulesets rule with the given settings.
func NewRepoRulesets(client *gh.Client, settings RulesetsSettings) *RepoRulesets {
	return &RepoRulesets{client: client, settings: settings}
}

func (r *RepoRulesets) Name() string { return "repo-rulesets" }

func (r *RepoRulesets) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] evaluating repository rulesets", repo.FullName())
	var allViolations []Violation

	for _, desired := range r.settings.Rulesets {
		actual, err := r.client.GetRepoRulesetByName(ctx, repo.Name, desired.Name)
		if err != nil {
			return nil, fmt.Errorf("fetching repo ruleset %q for %s: %w", desired.Name, repo.FullName(), err)
		}

		if actual == nil {
			allViolations = append(allViolations, Violation{
				Field:    desired.Name,
				Expected: "exists",
				Actual:   "not found",
				Message:  fmt.Sprintf("repo ruleset %q does not exist", desired.Name),
			})
			continue
		}

		allViolations = append(allViolations, checkRuleset(desired.Name, actual, desired)...)
	}

	return NewResult(r.Name(), repo.FullName(), allViolations), nil
}

func (r *RepoRulesets) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] applying repository rulesets", repo.FullName())

	for _, desired := range r.settings.Rulesets {
		actual, err := r.client.GetRepoRulesetByName(ctx, repo.Name, desired.Name)
		if err != nil {
			return nil, fmt.Errorf("fetching repo ruleset %q for %s: %w", desired.Name, repo.FullName(), err)
		}

		rs := buildRuleset(desired)

		if actual == nil {
			log.Printf("[%s] creating ruleset %q", repo.FullName(), desired.Name)
			_, err = r.client.CreateRepoRuleset(ctx, repo.Name, rs)
		} else {
			log.Printf("[%s] updating ruleset %q", repo.FullName(), desired.Name)
			_, err = r.client.UpdateRepoRuleset(ctx, repo.Name, actual.GetID(), rs)
		}

		if err != nil {
			return nil, fmt.Errorf("applying repo ruleset %q for %s: %w", desired.Name, repo.FullName(), err)
		}
	}

	result := NewResult(r.Name(), repo.FullName(), nil)
	result.Applied = true
	return result, nil
}

// ---------------------------------------------------------------------------
// Build helpers — convert config to go-github RepositoryRuleset
// ---------------------------------------------------------------------------

func buildRuleset(cfg RulesetConfig) gogithub.RepositoryRuleset {
	target := gogithub.RulesetTarget(cfg.Target)
	rs := gogithub.RepositoryRuleset{
		Name:        cfg.Name,
		Target:      &target,
		Enforcement: gogithub.RulesetEnforcement(cfg.Enforcement),
		Rules:       buildRules(cfg.Rules),
	}

	for _, ba := range cfg.BypassActors {
		actorID := ba.ActorID
		if actorID == 0 && ba.ActorType == "OrganizationAdmin" {
			actorID = 1
		}
		actorType := gogithub.BypassActorType(ba.ActorType)
		bypassMode := gogithub.BypassMode(ba.BypassMode)
		rs.BypassActors = append(rs.BypassActors, &gogithub.BypassActor{
			ActorID:    gogithub.Int64(actorID),
			ActorType:  &actorType,
			BypassMode: &bypassMode,
		})
	}

	if cfg.Conditions != nil {
		rs.Conditions = &gogithub.RepositoryRulesetConditions{}
		if cfg.Conditions.RefName != nil {
			rs.Conditions.RefName = &gogithub.RepositoryRulesetRefConditionParameters{
				Include: cfg.Conditions.RefName.Include,
				Exclude: cfg.Conditions.RefName.Exclude,
			}
		}
		if cfg.Conditions.RepositoryName != nil {
			rs.Conditions.RepositoryName = &gogithub.RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   cfg.Conditions.RepositoryName.Include,
				Exclude:   cfg.Conditions.RepositoryName.Exclude,
				Protected: cfg.Conditions.RepositoryName.Protected,
			}
		}
	}

	return rs
}

func buildRules(cfg RulesConfig) *gogithub.RepositoryRulesetRules {
	rules := &gogithub.RepositoryRulesetRules{}

	if cfg.Creation != nil && *cfg.Creation {
		rules.Creation = &gogithub.EmptyRuleParameters{}
	}
	if cfg.Update != nil && *cfg.Update {
		rules.Update = &gogithub.UpdateRuleParameters{}
	}
	if cfg.Deletion != nil && *cfg.Deletion {
		rules.Deletion = &gogithub.EmptyRuleParameters{}
	}
	if cfg.RequiredLinearHistory != nil && *cfg.RequiredLinearHistory {
		rules.RequiredLinearHistory = &gogithub.EmptyRuleParameters{}
	}
	if cfg.RequiredSignatures != nil && *cfg.RequiredSignatures {
		rules.RequiredSignatures = &gogithub.EmptyRuleParameters{}
	}
	if cfg.NonFastForward != nil && *cfg.NonFastForward {
		rules.NonFastForward = &gogithub.EmptyRuleParameters{}
	}

	if cfg.PullRequest != nil {
		rules.PullRequest = &gogithub.PullRequestRuleParameters{
			DismissStaleReviewsOnPush:      cfg.PullRequest.DismissStaleReviewsOnPush,
			RequireCodeOwnerReview:         cfg.PullRequest.RequireCodeOwnerReview,
			RequireLastPushApproval:        cfg.PullRequest.RequireLastPushApproval,
			RequiredApprovingReviewCount:   cfg.PullRequest.RequiredApprovingReviewCount,
			RequiredReviewThreadResolution: cfg.PullRequest.RequiredReviewThreadResolution,
		}
	}

	if cfg.RequiredStatusChecks != nil {
		var checks []*gogithub.RuleStatusCheck
		for _, sc := range cfg.RequiredStatusChecks.RequiredStatusChecks {
			checks = append(checks, &gogithub.RuleStatusCheck{
				Context:       sc.Context,
				IntegrationID: sc.IntegrationID,
			})
		}
		rules.RequiredStatusChecks = &gogithub.RequiredStatusChecksRuleParameters{
			StrictRequiredStatusChecksPolicy: cfg.RequiredStatusChecks.StrictRequiredStatusChecksPolicy,
			RequiredStatusChecks:             checks,
		}
	}

	if cfg.RequiredDeployments != nil {
		rules.RequiredDeployments = &gogithub.RequiredDeploymentsRuleParameters{
			RequiredDeploymentEnvironments: cfg.RequiredDeployments.RequiredDeploymentEnvironments,
		}
	}

	if cfg.CodeScanning != nil {
		var tools []*gogithub.RuleCodeScanningTool
		for _, t := range cfg.CodeScanning.CodeScanningTools {
			tools = append(tools, &gogithub.RuleCodeScanningTool{
				Tool:                    t.Tool,
				AlertsThreshold:         gogithub.CodeScanningAlertsThreshold(t.AlertsThreshold),
				SecurityAlertsThreshold: gogithub.CodeScanningSecurityAlertsThreshold(t.SecurityAlertsThreshold),
			})
		}
		rules.CodeScanning = &gogithub.CodeScanningRuleParameters{
			CodeScanningTools: tools,
		}
	}

	if cfg.CopilotCodeReview != nil {
		rules.CopilotCodeReview = &gogithub.CopilotCodeReviewRuleParameters{
			ReviewOnPush:            cfg.CopilotCodeReview.ReviewOnPush,
			ReviewDraftPullRequests: cfg.CopilotCodeReview.ReviewDraftPullRequests,
		}
	}

	return rules
}

// ---------------------------------------------------------------------------
// Evaluate helpers — compare actual ruleset against desired config
// ---------------------------------------------------------------------------

func checkRuleset(name string, actual *gogithub.RepositoryRuleset, desired RulesetConfig) []Violation {
	var violations []Violation
	prefix := name + "/"

	if actual.Enforcement != gogithub.RulesetEnforcement(desired.Enforcement) {
		violations = append(violations, Violation{
			Field:    prefix + "enforcement",
			Expected: desired.Enforcement,
			Actual:   string(actual.Enforcement),
		})
	}

	violations = append(violations, checkBypassActors(prefix, actual.BypassActors, desired.BypassActors)...)

	if desired.Conditions != nil {
		violations = append(violations, checkConditions(prefix, actual.Conditions, desired.Conditions)...)
	}

	violations = append(violations, checkRulesViolations(prefix, actual.Rules, desired.Rules)...)

	return violations
}

func checkBypassActors(prefix string, actual []*gogithub.BypassActor, desired []BypassActorConfig) []Violation {
	if len(desired) == 0 {
		return nil
	}

	type actorKey struct {
		actorID    int64
		actorType  string
		bypassMode string
	}
	actualSet := make(map[actorKey]bool, len(actual))
	for _, a := range actual {
		var at, bm string
		if a.ActorType != nil {
			at = string(*a.ActorType)
		}
		if a.BypassMode != nil {
			bm = string(*a.BypassMode)
		}
		actualSet[actorKey{a.GetActorID(), at, bm}] = true
	}

	var violations []Violation
	for _, d := range desired {
		id := d.ActorID
		if id == 0 && d.ActorType == "OrganizationAdmin" {
			id = 1
		}
		key := actorKey{id, d.ActorType, d.BypassMode}
		if !actualSet[key] {
			violations = append(violations, Violation{
				Field:    prefix + "bypass-actors",
				Expected: fmt.Sprintf("%s (bypass: %s)", d.ActorType, d.BypassMode),
				Actual:   "missing",
				Message:  fmt.Sprintf("bypass actor %s with mode %s not found", d.ActorType, d.BypassMode),
			})
		}
	}
	return violations
}

func checkConditions(prefix string, actual *gogithub.RepositoryRulesetConditions, desired *ConditionsConfig) []Violation {
	var violations []Violation

	if desired.RefName != nil {
		if actual == nil || actual.RefName == nil {
			violations = append(violations, Violation{
				Field:    prefix + "conditions/ref-name",
				Expected: "configured",
				Actual:   "not set",
			})
		} else {
			if missing := missingStrings(desired.RefName.Include, actual.RefName.Include); len(missing) > 0 {
				violations = append(violations, Violation{
					Field:    prefix + "conditions/ref-name/include",
					Expected: strings.Join(desired.RefName.Include, ", "),
					Actual:   strings.Join(actual.RefName.Include, ", "),
					Message:  fmt.Sprintf("missing ref includes: %s", strings.Join(missing, ", ")),
				})
			}
		}
	}

	if desired.RepositoryName != nil {
		if actual == nil || actual.RepositoryName == nil {
			violations = append(violations, Violation{
				Field:    prefix + "conditions/repository-name",
				Expected: "configured",
				Actual:   "not set",
			})
		} else {
			if missing := missingStrings(desired.RepositoryName.Include, actual.RepositoryName.Include); len(missing) > 0 {
				violations = append(violations, Violation{
					Field:    prefix + "conditions/repository-name/include",
					Expected: strings.Join(desired.RepositoryName.Include, ", "),
					Actual:   strings.Join(actual.RepositoryName.Include, ", "),
					Message:  fmt.Sprintf("missing repo includes: %s", strings.Join(missing, ", ")),
				})
			}
		}
	}

	return violations
}

func checkRulesViolations(prefix string, actual *gogithub.RepositoryRulesetRules, desired RulesConfig) []Violation {
	var violations []Violation

	if actual == nil {
		actual = &gogithub.RepositoryRulesetRules{}
	}

	checkToggle := func(field string, want *bool, present bool) {
		if want == nil {
			return
		}
		if *want && !present {
			violations = append(violations, Violation{
				Field:    prefix + "rules/" + field,
				Expected: "enabled",
				Actual:   "not present",
			})
		} else if !*want && present {
			violations = append(violations, Violation{
				Field:    prefix + "rules/" + field,
				Expected: "disabled",
				Actual:   "present",
			})
		}
	}

	checkToggle("creation", desired.Creation, actual.Creation != nil)
	checkToggle("update", desired.Update, actual.Update != nil)
	checkToggle("deletion", desired.Deletion, actual.Deletion != nil)
	checkToggle("required-linear-history", desired.RequiredLinearHistory, actual.RequiredLinearHistory != nil)
	checkToggle("required-signatures", desired.RequiredSignatures, actual.RequiredSignatures != nil)
	checkToggle("non-fast-forward", desired.NonFastForward, actual.NonFastForward != nil)

	if desired.PullRequest != nil {
		if actual.PullRequest == nil {
			violations = append(violations, Violation{
				Field:    prefix + "rules/pull-request",
				Expected: "enabled",
				Actual:   "not present",
			})
		} else {
			violations = append(violations, checkPullRequestParams(prefix, actual.PullRequest, desired.PullRequest)...)
		}
	}

	if desired.RequiredStatusChecks != nil {
		if actual.RequiredStatusChecks == nil {
			violations = append(violations, Violation{
				Field:    prefix + "rules/required-status-checks",
				Expected: "enabled",
				Actual:   "not present",
			})
		} else {
			violations = append(violations, checkStatusChecksParams(prefix, actual.RequiredStatusChecks, desired.RequiredStatusChecks)...)
		}
	}

	if desired.RequiredDeployments != nil {
		if actual.RequiredDeployments == nil {
			violations = append(violations, Violation{
				Field:    prefix + "rules/required-deployments",
				Expected: "enabled",
				Actual:   "not present",
			})
		} else {
			violations = append(violations, checkDeploymentsParams(prefix, actual.RequiredDeployments, desired.RequiredDeployments)...)
		}
	}

	if desired.CodeScanning != nil {
		if actual.CodeScanning == nil {
			violations = append(violations, Violation{
				Field:    prefix + "rules/code-scanning",
				Expected: "enabled",
				Actual:   "not present",
			})
		} else {
			violations = append(violations, checkCodeScanningParams(prefix, actual.CodeScanning, desired.CodeScanning)...)
		}
	}

	if desired.CopilotCodeReview != nil {
		if actual.CopilotCodeReview == nil {
			violations = append(violations, Violation{
				Field:    prefix + "rules/copilot-code-review",
				Expected: "enabled",
				Actual:   "not present",
			})
		} else {
			violations = append(violations, checkCopilotCodeReviewParams(prefix, actual.CopilotCodeReview, desired.CopilotCodeReview)...)
		}
	}

	return violations
}

func checkPullRequestParams(prefix string, actual *gogithub.PullRequestRuleParameters, desired *PullRequestRuleConfig) []Violation {
	var violations []Violation
	p := prefix + "rules/pull-request/"

	if actual.DismissStaleReviewsOnPush != desired.DismissStaleReviewsOnPush {
		violations = append(violations, Violation{
			Field:    p + "dismiss-stale-reviews-on-push",
			Expected: fmt.Sprintf("%t", desired.DismissStaleReviewsOnPush),
			Actual:   fmt.Sprintf("%t", actual.DismissStaleReviewsOnPush),
		})
	}
	if actual.RequireCodeOwnerReview != desired.RequireCodeOwnerReview {
		violations = append(violations, Violation{
			Field:    p + "require-code-owner-review",
			Expected: fmt.Sprintf("%t", desired.RequireCodeOwnerReview),
			Actual:   fmt.Sprintf("%t", actual.RequireCodeOwnerReview),
		})
	}
	if actual.RequireLastPushApproval != desired.RequireLastPushApproval {
		violations = append(violations, Violation{
			Field:    p + "require-last-push-approval",
			Expected: fmt.Sprintf("%t", desired.RequireLastPushApproval),
			Actual:   fmt.Sprintf("%t", actual.RequireLastPushApproval),
		})
	}
	if actual.RequiredApprovingReviewCount != desired.RequiredApprovingReviewCount {
		violations = append(violations, Violation{
			Field:    p + "required-approving-review-count",
			Expected: fmt.Sprintf("%d", desired.RequiredApprovingReviewCount),
			Actual:   fmt.Sprintf("%d", actual.RequiredApprovingReviewCount),
		})
	}
	if actual.RequiredReviewThreadResolution != desired.RequiredReviewThreadResolution {
		violations = append(violations, Violation{
			Field:    p + "required-review-thread-resolution",
			Expected: fmt.Sprintf("%t", desired.RequiredReviewThreadResolution),
			Actual:   fmt.Sprintf("%t", actual.RequiredReviewThreadResolution),
		})
	}
	return violations
}

func checkStatusChecksParams(prefix string, actual *gogithub.RequiredStatusChecksRuleParameters, desired *RequiredStatusChecksRuleConfig) []Violation {
	var violations []Violation
	p := prefix + "rules/required-status-checks/"

	if actual.StrictRequiredStatusChecksPolicy != desired.StrictRequiredStatusChecksPolicy {
		violations = append(violations, Violation{
			Field:    p + "strict-required-status-checks-policy",
			Expected: fmt.Sprintf("%t", desired.StrictRequiredStatusChecksPolicy),
			Actual:   fmt.Sprintf("%t", actual.StrictRequiredStatusChecksPolicy),
		})
	}

	actualContexts := make(map[string]bool, len(actual.RequiredStatusChecks))
	for _, sc := range actual.RequiredStatusChecks {
		actualContexts[sc.Context] = true
	}
	var desiredContexts, missingContexts []string
	for _, sc := range desired.RequiredStatusChecks {
		desiredContexts = append(desiredContexts, sc.Context)
		if !actualContexts[sc.Context] {
			missingContexts = append(missingContexts, sc.Context)
		}
	}
	if len(missingContexts) > 0 {
		sort.Strings(missingContexts)
		var actualList []string
		for _, sc := range actual.RequiredStatusChecks {
			actualList = append(actualList, sc.Context)
		}
		sort.Strings(actualList)
		violations = append(violations, Violation{
			Field:    p + "required-status-checks",
			Expected: strings.Join(desiredContexts, ", "),
			Actual:   strings.Join(actualList, ", "),
			Message:  fmt.Sprintf("missing required checks: %s", strings.Join(missingContexts, ", ")),
		})
	}

	return violations
}

func checkDeploymentsParams(prefix string, actual *gogithub.RequiredDeploymentsRuleParameters, desired *RequiredDeploymentsRuleConfig) []Violation {
	missing := missingStrings(desired.RequiredDeploymentEnvironments, actual.RequiredDeploymentEnvironments)
	if len(missing) > 0 {
		return []Violation{{
			Field:    prefix + "rules/required-deployments/environments",
			Expected: strings.Join(desired.RequiredDeploymentEnvironments, ", "),
			Actual:   strings.Join(actual.RequiredDeploymentEnvironments, ", "),
			Message:  fmt.Sprintf("missing deployment environments: %s", strings.Join(missing, ", ")),
		}}
	}
	return nil
}

func checkCodeScanningParams(prefix string, actual *gogithub.CodeScanningRuleParameters, desired *CodeScanningRuleConfig) []Violation {
	actualTools := make(map[string]*gogithub.RuleCodeScanningTool, len(actual.CodeScanningTools))
	for _, t := range actual.CodeScanningTools {
		actualTools[t.Tool] = t
	}

	var violations []Violation
	p := prefix + "rules/code-scanning/"

	for _, dt := range desired.CodeScanningTools {
		at, exists := actualTools[dt.Tool]
		if !exists {
			violations = append(violations, Violation{
				Field:    p + "tools",
				Expected: dt.Tool,
				Actual:   "missing",
				Message:  fmt.Sprintf("code scanning tool %q not found", dt.Tool),
			})
			continue
		}
		if string(at.AlertsThreshold) != dt.AlertsThreshold {
			violations = append(violations, Violation{
				Field:    p + dt.Tool + "/alerts-threshold",
				Expected: dt.AlertsThreshold,
				Actual:   string(at.AlertsThreshold),
			})
		}
		if string(at.SecurityAlertsThreshold) != dt.SecurityAlertsThreshold {
			violations = append(violations, Violation{
				Field:    p + dt.Tool + "/security-alerts-threshold",
				Expected: dt.SecurityAlertsThreshold,
				Actual:   string(at.SecurityAlertsThreshold),
			})
		}
	}

	return violations
}

func checkCopilotCodeReviewParams(prefix string, actual *gogithub.CopilotCodeReviewRuleParameters, desired *CopilotCodeReviewRuleConfig) []Violation {
	var violations []Violation
	p := prefix + "rules/copilot-code-review/"

	if actual.ReviewOnPush != desired.ReviewOnPush {
		violations = append(violations, Violation{
			Field:    p + "review-on-push",
			Expected: fmt.Sprintf("%t", desired.ReviewOnPush),
			Actual:   fmt.Sprintf("%t", actual.ReviewOnPush),
		})
	}
	if actual.ReviewDraftPullRequests != desired.ReviewDraftPullRequests {
		violations = append(violations, Violation{
			Field:    p + "review-draft-pull-requests",
			Expected: fmt.Sprintf("%t", desired.ReviewDraftPullRequests),
			Actual:   fmt.Sprintf("%t", actual.ReviewDraftPullRequests),
		})
	}
	return violations
}

// missingStrings returns required strings that are not present in actual.
func missingStrings(required, actual []string) []string {
	have := make(map[string]bool, len(actual))
	for _, s := range actual {
		have[s] = true
	}
	var missing []string
	for _, s := range required {
		if !have[s] {
			missing = append(missing, s)
		}
	}
	sort.Strings(missing)
	return missing
}
