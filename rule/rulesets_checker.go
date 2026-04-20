package rule

import (
	"fmt"
	"sort"
	"strings"

	gogithub "github.com/google/go-github/v84/github"
)

// rulesetChecker accumulates violations with a shared field prefix.
type rulesetChecker struct {
	prefix     string
	violations []Violation
}

// checkRuleset compares an actual GitHub ruleset against the desired config
// and returns any violations found.
func checkRuleset(name string, actual *gogithub.RepositoryRuleset, desired RulesetConfig) []Violation {
	c := &rulesetChecker{prefix: name + "/"}

	c.checkStr("enforcement", desired.Enforcement, string(actual.Enforcement))
	c.checkBypassActors(actual.BypassActors, desired.BypassActors)
	if desired.Conditions != nil {
		c.checkConditions(actual.Conditions, desired.Conditions)
	}
	c.checkRules(actual.Rules, desired.Rules)

	return c.violations
}

// checkBool records a violation if the expected and actual booleans differ.
func (c *rulesetChecker) checkBool(name string, expected, actual bool) {
	if expected != actual {
		c.violations = append(c.violations, Violation{
			Field:    c.prefix + name,
			Expected: fmt.Sprintf("%t", expected),
			Actual:   fmt.Sprintf("%t", actual),
		})
	}
}

// checkInt records a violation if the expected and actual integers differ.
func (c *rulesetChecker) checkInt(name string, expected, actual int) {
	if expected != actual {
		c.violations = append(c.violations, Violation{
			Field:    c.prefix + name,
			Expected: fmt.Sprintf("%d", expected),
			Actual:   fmt.Sprintf("%d", actual),
		})
	}
}

// checkStr records a violation if the expected and actual strings differ.
func (c *rulesetChecker) checkStr(name string, expected, actual string) {
	if expected != actual {
		c.violations = append(c.violations, Violation{
			Field:    c.prefix + name,
			Expected: expected,
			Actual:   actual,
		})
	}
}

// checkToggle records a violation for toggle rules (enabled/disabled).
// A nil want means the rule is unmanaged and is skipped.
func (c *rulesetChecker) checkToggle(name string, want *bool, present bool) {
	if want == nil {
		return
	}
	if *want && !present {
		c.violations = append(c.violations, Violation{
			Field: c.prefix + name, Expected: "enabled", Actual: "not present",
		})
	} else if !*want && present {
		c.violations = append(c.violations, Violation{
			Field: c.prefix + name, Expected: "disabled", Actual: "present",
		})
	}
}

// checkMissing records a violation if any required strings are absent from actual.
func (c *rulesetChecker) checkMissing(name string, required, actual []string) {
	missing := missingStrings(required, actual)
	if len(missing) > 0 {
		c.violations = append(c.violations, Violation{
			Field:    c.prefix + name,
			Expected: strings.Join(required, ", "),
			Actual:   strings.Join(actual, ", "),
			Message:  fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		})
	}
}

// absent records a violation for a rule that should exist but is not present.
func (c *rulesetChecker) absent(name string) {
	c.violations = append(c.violations, Violation{
		Field: c.prefix + name, Expected: "enabled", Actual: "not present",
	})
}

// checkBypassActors verifies that all desired bypass actors exist in the actual ruleset.
func (c *rulesetChecker) checkBypassActors(actual []*gogithub.BypassActor, desired []BypassActorConfig) {
	if len(desired) == 0 {
		return
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

	for _, d := range desired {
		id := d.ActorID
		if id == 0 && d.ActorType == "OrganizationAdmin" {
			id = 1
		}
		if !actualSet[actorKey{id, d.ActorType, d.BypassMode}] {
			c.violations = append(c.violations, Violation{
				Field:    c.prefix + "bypass-actors",
				Expected: fmt.Sprintf("%s (bypass: %s)", d.ActorType, d.BypassMode),
				Actual:   "missing",
				Message:  fmt.Sprintf("bypass actor %s with mode %s not found", d.ActorType, d.BypassMode),
			})
		}
	}
}

// checkConditions verifies ref-name and repository-name targeting conditions.
func (c *rulesetChecker) checkConditions(actual *gogithub.RepositoryRulesetConditions, desired *ConditionsConfig) {
	if desired.RefName != nil {
		if actual == nil || actual.RefName == nil {
			c.violations = append(c.violations, Violation{
				Field: c.prefix + "conditions/ref-name", Expected: "configured", Actual: "not set",
			})
		} else {
			c.checkMissing("conditions/ref-name/include", desired.RefName.Include, actual.RefName.Include)
		}
	}

	if desired.RepositoryName != nil {
		if actual == nil || actual.RepositoryName == nil {
			c.violations = append(c.violations, Violation{
				Field: c.prefix + "conditions/repository-name", Expected: "configured", Actual: "not set",
			})
		} else {
			c.checkMissing("conditions/repository-name/include", desired.RepositoryName.Include, actual.RepositoryName.Include)
		}
	}
}

// checkRules compares all individual rules within a ruleset against the desired config.
func (c *rulesetChecker) checkRules(actual *gogithub.RepositoryRulesetRules, desired RulesConfig) {
	if actual == nil {
		actual = &gogithub.RepositoryRulesetRules{}
	}

	// Toggle rules
	c.checkToggle("rules/creation", desired.Creation, actual.Creation != nil)
	c.checkToggle("rules/update", desired.Update, actual.Update != nil)
	c.checkToggle("rules/deletion", desired.Deletion, actual.Deletion != nil)
	c.checkToggle("rules/require-linear-history", desired.RequireLinearHistory, actual.RequiredLinearHistory != nil)
	c.checkToggle("rules/require-signed-commits", desired.RequireSignedCommits, actual.RequiredSignatures != nil)
	c.checkToggle("rules/block-force-pushes", desired.BlockForcePushes, actual.NonFastForward != nil)

	// Pull request
	if desired.PullRequest != nil {
		if actual.PullRequest == nil {
			c.absent("rules/pull-request")
		} else {
			pr := desired.PullRequest
			c.checkBool("rules/pull-request/dismiss-stale-approvals-on-push", pr.DismissStaleApprovalsOnPush, actual.PullRequest.DismissStaleReviewsOnPush)
			c.checkBool("rules/pull-request/require-code-owner-review", pr.RequireCodeOwnerReview, actual.PullRequest.RequireCodeOwnerReview)
			c.checkBool("rules/pull-request/require-most-recent-push-approval", pr.RequireMostRecentPushApproval, actual.PullRequest.RequireLastPushApproval)
			c.checkInt("rules/pull-request/required-approvals", pr.RequiredApprovals, actual.PullRequest.RequiredApprovingReviewCount)
			c.checkBool("rules/pull-request/require-conversation-resolution", pr.RequireConversationResolution, actual.PullRequest.RequiredReviewThreadResolution)

			if len(pr.AllowedMergeMethods) > 0 {
				var actualMethods []string
				for _, m := range actual.PullRequest.AllowedMergeMethods {
					actualMethods = append(actualMethods, string(m))
				}
				c.checkMissing("rules/pull-request/allowed-merge-methods", pr.AllowedMergeMethods, actualMethods)
			}

			c.checkRequiredReviewers(pr.RequiredReviewers, actual.PullRequest.RequiredReviewers)
		}
	}

	// Required status checks
	if desired.RequiredStatusChecks != nil {
		if actual.RequiredStatusChecks == nil {
			c.absent("rules/required-status-checks")
		} else {
			c.checkBool("rules/required-status-checks/require-branches-to-be-up-to-date",
				desired.RequiredStatusChecks.RequireBranchesToBeUpToDate,
				actual.RequiredStatusChecks.StrictRequiredStatusChecksPolicy)
			c.checkBool("rules/required-status-checks/do-not-require-status-checks-on-creation",
				desired.RequiredStatusChecks.DoNotRequireStatusChecksOnCreate,
				actual.RequiredStatusChecks.GetDoNotEnforceOnCreate())

			var desiredCtx, actualCtx []string
			for _, s := range desired.RequiredStatusChecks.RequiredStatusChecks {
				desiredCtx = append(desiredCtx, s.Context)
			}
			for _, s := range actual.RequiredStatusChecks.RequiredStatusChecks {
				actualCtx = append(actualCtx, s.Context)
			}
			c.checkMissing("rules/required-status-checks/contexts", desiredCtx, actualCtx)
		}
	}

	// Required deployments
	if desired.RequiredDeployments != nil {
		if actual.RequiredDeployments == nil {
			c.absent("rules/required-deployments")
		} else {
			c.checkMissing("rules/required-deployments/environments",
				desired.RequiredDeployments.RequiredDeploymentEnvironments,
				actual.RequiredDeployments.RequiredDeploymentEnvironments)
		}
	}

	// Merge queue
	if desired.MergeQueue != nil {
		if actual.MergeQueue == nil {
			c.absent("rules/merge-queue")
		} else {
			c.checkStr("rules/merge-queue/merge-method", desired.MergeQueue.MergeMethod, string(actual.MergeQueue.MergeMethod))
			c.checkStr("rules/merge-queue/grouping-strategy", desired.MergeQueue.GroupingStrategy, string(actual.MergeQueue.GroupingStrategy))
			c.checkInt("rules/merge-queue/check-response-timeout-minutes", desired.MergeQueue.CheckResponseTimeoutMinutes, actual.MergeQueue.CheckResponseTimeoutMinutes)
			c.checkInt("rules/merge-queue/max-entries-to-build", desired.MergeQueue.MaxEntriesToBuild, actual.MergeQueue.MaxEntriesToBuild)
			c.checkInt("rules/merge-queue/max-entries-to-merge", desired.MergeQueue.MaxEntriesToMerge, actual.MergeQueue.MaxEntriesToMerge)
			c.checkInt("rules/merge-queue/min-entries-to-merge", desired.MergeQueue.MinEntriesToMerge, actual.MergeQueue.MinEntriesToMerge)
			c.checkInt("rules/merge-queue/min-entries-to-merge-wait-minutes", desired.MergeQueue.MinEntriesToMergeWaitMinutes, actual.MergeQueue.MinEntriesToMergeWaitMinutes)
		}
	}

	// Code scanning
	if desired.CodeScanning != nil {
		if actual.CodeScanning == nil {
			c.absent("rules/code-scanning")
		} else {
			actualTools := make(map[string]*gogithub.RuleCodeScanningTool, len(actual.CodeScanning.CodeScanningTools))
			for _, t := range actual.CodeScanning.CodeScanningTools {
				actualTools[t.Tool] = t
			}
			for _, dt := range desired.CodeScanning.CodeScanningTools {
				at, exists := actualTools[dt.Tool]
				if !exists {
					c.violations = append(c.violations, Violation{
						Field: c.prefix + "rules/code-scanning/tools", Expected: dt.Tool, Actual: "missing",
						Message: fmt.Sprintf("code scanning tool %q not found", dt.Tool),
					})
					continue
				}
				c.checkStr("rules/code-scanning/"+dt.Tool+"/alerts-threshold", dt.AlertsThreshold, string(at.AlertsThreshold))
				c.checkStr("rules/code-scanning/"+dt.Tool+"/security-alerts-threshold", dt.SecurityAlertsThreshold, string(at.SecurityAlertsThreshold))
			}
		}
	}

	// Copilot code review
	if desired.CopilotCodeReview != nil {
		if actual.CopilotCodeReview == nil {
			c.absent("rules/copilot-code-review")
		} else {
			c.checkBool("rules/copilot-code-review/review-on-push", desired.CopilotCodeReview.ReviewOnPush, actual.CopilotCodeReview.ReviewOnPush)
			c.checkBool("rules/copilot-code-review/review-draft-pull-requests", desired.CopilotCodeReview.ReviewDraftPullRequests, actual.CopilotCodeReview.ReviewDraftPullRequests)
		}
	}

	// Pattern rules
	if desired.CommitMessagePattern != nil {
		if actual.CommitMessagePattern == nil {
			c.absent("rules/commit-message-pattern")
		} else {
			c.checkPatternRule("rules/commit-message-pattern", desired.CommitMessagePattern, actual.CommitMessagePattern)
		}
	}

	if desired.BranchNamePattern != nil {
		if actual.BranchNamePattern == nil {
			c.absent("rules/branch-name-pattern")
		} else {
			c.checkPatternRule("rules/branch-name-pattern", desired.BranchNamePattern, actual.BranchNamePattern)
		}
	}
}

// checkPatternRule compares a pattern-based rule (commit message, branch name) against actual.
func (c *rulesetChecker) checkPatternRule(prefix string, desired *PatternRuleConfig, actual *gogithub.PatternRuleParameters) {
	c.checkStr(prefix+"/operator", desired.Operator, string(actual.Operator))
	c.checkStr(prefix+"/pattern", desired.Pattern, actual.Pattern)
	c.checkBool(prefix+"/negate", desired.Negate, actual.GetNegate())
}

// checkRequiredReviewers verifies team-based required reviewers, detecting missing
// and unexpected teams.
func (c *rulesetChecker) checkRequiredReviewers(desired []RequiredReviewerConfig, actual []*gogithub.RulesetRequiredReviewer) {
	actualByTeam := make(map[int64]*gogithub.RulesetRequiredReviewer, len(actual))
	for _, r := range actual {
		if r.Reviewer != nil && r.Reviewer.ID != nil {
			actualByTeam[*r.Reviewer.ID] = r
		}
	}

	for _, d := range desired {
		ar, found := actualByTeam[d.TeamID]
		if !found {
			c.absent(fmt.Sprintf("rules/pull-request/required-reviewers/team-%d", d.TeamID))
			continue
		}
		prefix := fmt.Sprintf("rules/pull-request/required-reviewers/team-%d", d.TeamID)
		c.checkInt(prefix+"/minimum-approvals", d.MinimumApprovals, ar.GetMinimumApprovals())
		if len(d.FilePatterns) > 0 {
			c.checkMissing(prefix+"/file-patterns", d.FilePatterns, ar.FilePatterns)
		}
		delete(actualByTeam, d.TeamID)
	}

	for teamID := range actualByTeam {
		c.violations = append(c.violations, Violation{
			Field:    fmt.Sprintf("rules/pull-request/required-reviewers/team-%d", teamID),
			Expected: "absent",
			Actual:   "present",
			Message:  fmt.Sprintf("unexpected required reviewer team %d not defined in config", teamID),
		})
	}
}

// missingStrings returns sorted strings present in required but absent from actual.
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
