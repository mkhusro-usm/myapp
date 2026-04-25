package rule

import (
	"context"
	"fmt"
	"log"
	"strings"

	gogithub "github.com/google/go-github/v84/github"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// RulesetsSettings is the top-level settings for the rulesets governance rule.
type RulesetsSettings struct {
	Rulesets []RulesetConfig `yaml:"rulesets"`
}

// RulesetConfig describes a single desired ruleset.
// The Name field supports a {default_branch} placeholder that is resolved
// at evaluate/apply time using each repository's actual default branch.
type RulesetConfig struct {
	Name         string              `yaml:"name"`
	Target       string              `yaml:"target"`      // "branch" or "tag"
	Enforcement  string              `yaml:"enforcement"` // "active", "disabled"
	BypassActors []BypassActorConfig `yaml:"bypass-actors"`
	Conditions   *ConditionsConfig   `yaml:"conditions"`
	Rules        RulesConfig         `yaml:"rules"`
}

// BypassActorConfig describes an actor allowed to bypass the ruleset.
type BypassActorConfig struct {
	ActorID    int64  `yaml:"actor-id"`
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
	// Toggle rules (ordered per GitHub rulesets UI)
	Creation             *bool `yaml:"creation"`
	Update               *bool `yaml:"update"`
	Deletion             *bool `yaml:"deletion"`
	RequireLinearHistory *bool `yaml:"require-linear-history"`
	RequireSignedCommits *bool `yaml:"require-signed-commits"`
	BlockForcePushes     *bool `yaml:"block-force-pushes"`

	// Parameterized rules (ordered per GitHub rulesets UI)
	RequiredDeployments  *RequiredDeploymentsRuleConfig  `yaml:"require-deployments-to-succeed"`
	PullRequest          *PullRequestRuleConfig          `yaml:"pull-request"`
	RequiredStatusChecks *RequiredStatusChecksRuleConfig `yaml:"required-status-checks"`
	CodeScanning         *CodeScanningRuleConfig         `yaml:"code-scanning"`
	CopilotCodeReview    *CopilotCodeReviewRuleConfig    `yaml:"copilot-code-review"`
	MergeQueue           *MergeQueueRuleConfig           `yaml:"merge-queue"`

	// Pattern rules
	CommitMessagePattern *PatternRuleConfig `yaml:"commit-message-pattern"`
	BranchNamePattern    *PatternRuleConfig `yaml:"branch-name-pattern"`
}

// PullRequestRuleConfig maps to the pull_request rule parameters.
type PullRequestRuleConfig struct {
	RequiredApprovals             int                      `yaml:"required-approvals"`
	DismissStaleApprovalsOnPush   bool                     `yaml:"dismiss-stale-approvals-on-push"`
	RequireCodeOwnerReview        bool                     `yaml:"require-code-owner-review"`
	RequireMostRecentPushApproval bool                     `yaml:"require-most-recent-push-approval"`
	RequireConversationResolution bool                     `yaml:"require-conversation-resolution"`
	RequiredReviewers             []RequiredReviewerConfig `yaml:"required-reviewers"`
	AllowedMergeMethods           []string                 `yaml:"allowed-merge-methods"`
}

// RequiredReviewerConfig maps to the "Require review from specific teams" setting.
type RequiredReviewerConfig struct {
	TeamID           int64    `yaml:"team-id"`
	MinimumApprovals int      `yaml:"minimum-approvals"`
	FilePatterns     []string `yaml:"file-patterns"`
}

// RequiredStatusChecksRuleConfig maps to the required_status_checks rule parameters.
type RequiredStatusChecksRuleConfig struct {
	RequireBranchesToBeUpToDate      bool                `yaml:"require-branches-to-be-up-to-date"`
	DoNotRequireStatusChecksOnCreate bool                `yaml:"do-not-require-status-checks-on-creation"`
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

// MergeQueueRuleConfig maps to the merge_queue rule parameters.
type MergeQueueRuleConfig struct {
	CheckResponseTimeoutMinutes  int    `yaml:"check-response-timeout-minutes"`
	GroupingStrategy             string `yaml:"grouping-strategy"`
	MaxEntriesToBuild            int    `yaml:"max-entries-to-build"`
	MaxEntriesToMerge            int    `yaml:"max-entries-to-merge"`
	MergeMethod                  string `yaml:"merge-method"`
	MinEntriesToMerge            int    `yaml:"min-entries-to-merge"`
	MinEntriesToMergeWaitMinutes int    `yaml:"min-entries-to-merge-wait-minutes"`
}

// PatternRuleConfig maps to pattern-based rule parameters (commit message, branch name).
type PatternRuleConfig struct {
	Name     string `yaml:"name"`
	Negate   bool   `yaml:"negate"`
	Operator string `yaml:"operator"` // starts_with, ends_with, contains, regex
	Pattern  string `yaml:"pattern"`
}

// RepoRulesets enforces rulesets at the repository level.
type RepoRulesets struct {
	client   RulesetsClient
	settings RulesetsSettings
}

// NewRepoRulesets creates a RepoRulesets rule with the given settings.
func NewRepoRulesets(client RulesetsClient, settings RulesetsSettings) *RepoRulesets {
	return &RepoRulesets{client: client, settings: settings}
}

// Name returns the rule identifier.
func (r *RepoRulesets) Name() string { return "repo-rulesets" }

// Evaluate checks whether the repository's rulesets match the desired config.
func (r *RepoRulesets) Evaluate(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] evaluating repository rulesets", repo.FullName())
	var allViolations []Violation

	for _, desired := range r.settings.Rulesets {
		desired = desired.withResolvedName(repo.DefaultBranch)

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

// Apply creates or updates rulesets on the repository to match the desired config.
func (r *RepoRulesets) Apply(ctx context.Context, repo *gh.Repository) (*Result, error) {
	log.Printf("[%s] applying repository rulesets", repo.FullName())

	for _, desired := range r.settings.Rulesets {
		desired = desired.withResolvedName(repo.DefaultBranch)

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

// withResolvedName returns a copy with {default_branch} replaced by the actual branch name.
func (rc RulesetConfig) withResolvedName(defaultBranch string) RulesetConfig {
	rc.Name = strings.ReplaceAll(rc.Name, "{default_branch}", defaultBranch)
	return rc
}

// buildRuleset converts a RulesetConfig into a go-github RepositoryRuleset for API calls.
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

// buildRules converts the rules config into go-github rule parameters.
func buildRules(cfg RulesConfig) *gogithub.RepositoryRulesetRules {
	rules := &gogithub.RepositoryRulesetRules{}

	// Toggle rules
	if cfg.Creation != nil && *cfg.Creation {
		rules.Creation = &gogithub.EmptyRuleParameters{}
	}
	if cfg.Update != nil && *cfg.Update {
		rules.Update = &gogithub.UpdateRuleParameters{}
	}
	if cfg.Deletion != nil && *cfg.Deletion {
		rules.Deletion = &gogithub.EmptyRuleParameters{}
	}
	if cfg.RequireLinearHistory != nil && *cfg.RequireLinearHistory {
		rules.RequiredLinearHistory = &gogithub.EmptyRuleParameters{}
	}
	if cfg.RequireSignedCommits != nil && *cfg.RequireSignedCommits {
		rules.RequiredSignatures = &gogithub.EmptyRuleParameters{}
	}
	if cfg.BlockForcePushes != nil && *cfg.BlockForcePushes {
		rules.NonFastForward = &gogithub.EmptyRuleParameters{}
	}

	// Required deployments
	if cfg.RequiredDeployments != nil {
		rules.RequiredDeployments = &gogithub.RequiredDeploymentsRuleParameters{
			RequiredDeploymentEnvironments: cfg.RequiredDeployments.RequiredDeploymentEnvironments,
		}
	}

	// Pull request
	if cfg.PullRequest != nil {
		pr := &gogithub.PullRequestRuleParameters{
			DismissStaleReviewsOnPush:      cfg.PullRequest.DismissStaleApprovalsOnPush,
			RequireCodeOwnerReview:         cfg.PullRequest.RequireCodeOwnerReview,
			RequireLastPushApproval:        cfg.PullRequest.RequireMostRecentPushApproval,
			RequiredApprovingReviewCount:   cfg.PullRequest.RequiredApprovals,
			RequiredReviewThreadResolution: cfg.PullRequest.RequireConversationResolution,
		}
		for _, m := range cfg.PullRequest.AllowedMergeMethods {
			pr.AllowedMergeMethods = append(pr.AllowedMergeMethods, gogithub.PullRequestMergeMethod(m))
		}
		for _, r := range cfg.PullRequest.RequiredReviewers {
			reviewerType := gogithub.RulesetReviewerTypeTeam
			pr.RequiredReviewers = append(pr.RequiredReviewers, &gogithub.RulesetRequiredReviewer{
				MinimumApprovals: &r.MinimumApprovals,
				FilePatterns:     r.FilePatterns,
				Reviewer: &gogithub.RulesetReviewer{
					ID:   &r.TeamID,
					Type: &reviewerType,
				},
			})
		}
		rules.PullRequest = pr
	}

	// Required status checks — GitHub API rejects this rule with an empty checks list,
	// so only emit it when at least one check is configured.
	if cfg.RequiredStatusChecks != nil && len(cfg.RequiredStatusChecks.RequiredStatusChecks) > 0 {
		var checks []*gogithub.RuleStatusCheck
		for _, sc := range cfg.RequiredStatusChecks.RequiredStatusChecks {
			checks = append(checks, &gogithub.RuleStatusCheck{
				Context:       sc.Context,
				IntegrationID: sc.IntegrationID,
			})
		}
		rules.RequiredStatusChecks = &gogithub.RequiredStatusChecksRuleParameters{
			StrictRequiredStatusChecksPolicy: cfg.RequiredStatusChecks.RequireBranchesToBeUpToDate,
			DoNotEnforceOnCreate:             &cfg.RequiredStatusChecks.DoNotRequireStatusChecksOnCreate,
			RequiredStatusChecks:             checks,
		}
	}

	// Code scanning
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

	// Copilot code review
	if cfg.CopilotCodeReview != nil {
		rules.CopilotCodeReview = &gogithub.CopilotCodeReviewRuleParameters{
			ReviewOnPush:            cfg.CopilotCodeReview.ReviewOnPush,
			ReviewDraftPullRequests: cfg.CopilotCodeReview.ReviewDraftPullRequests,
		}
	}

	// Merge queue
	if cfg.MergeQueue != nil {
		rules.MergeQueue = &gogithub.MergeQueueRuleParameters{
			CheckResponseTimeoutMinutes:  cfg.MergeQueue.CheckResponseTimeoutMinutes,
			GroupingStrategy:             gogithub.MergeGroupingStrategy(cfg.MergeQueue.GroupingStrategy),
			MaxEntriesToBuild:            cfg.MergeQueue.MaxEntriesToBuild,
			MaxEntriesToMerge:            cfg.MergeQueue.MaxEntriesToMerge,
			MergeMethod:                  gogithub.MergeQueueMergeMethod(cfg.MergeQueue.MergeMethod),
			MinEntriesToMerge:            cfg.MergeQueue.MinEntriesToMerge,
			MinEntriesToMergeWaitMinutes: cfg.MergeQueue.MinEntriesToMergeWaitMinutes,
		}
	}

	// Pattern rules
	if cfg.CommitMessagePattern != nil {
		rules.CommitMessagePattern = buildPatternRule(cfg.CommitMessagePattern)
	}
	if cfg.BranchNamePattern != nil {
		rules.BranchNamePattern = buildPatternRule(cfg.BranchNamePattern)
	}

	return rules
}

// buildPatternRule converts a pattern rule config into go-github parameters.
func buildPatternRule(cfg *PatternRuleConfig) *gogithub.PatternRuleParameters {
	return &gogithub.PatternRuleParameters{
		Name:     gogithub.Ptr(cfg.Name),
		Negate:   gogithub.Ptr(cfg.Negate),
		Operator: gogithub.PatternRuleOperator(cfg.Operator),
		Pattern:  cfg.Pattern,
	}
}
