package rule

import (
	"testing"

	gogithub "github.com/google/go-github/v84/github"
)

func TestMissingStrings(t *testing.T) {
	t.Run("none missing", func(t *testing.T) {
		got := missingStrings([]string{"a", "b"}, []string{"a", "b", "c"})
		if len(got) != 0 {
			t.Errorf("missing = %v, want empty", got)
		}
	})

	t.Run("some missing", func(t *testing.T) {
		got := missingStrings([]string{"a", "b", "c"}, []string{"b"})
		if len(got) != 2 {
			t.Fatalf("missing count = %d, want 2", len(got))
		}
		if got[0] != "a" || got[1] != "c" {
			t.Errorf("missing = %v, want [a c]", got)
		}
	})

	t.Run("all missing", func(t *testing.T) {
		got := missingStrings([]string{"x", "y"}, nil)
		if len(got) != 2 {
			t.Errorf("missing count = %d, want 2", len(got))
		}
	})
}

func TestCheckerPrimitives(t *testing.T) {
	t.Run("checkBool match", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkBool("field", true, true)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("checkBool mismatch", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkBool("field", true, false)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Field != "test/field" {
			t.Errorf("Field = %q, want %q", c.violations[0].Field, "test/field")
		}
	})

	t.Run("checkInt match", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkInt("count", 5, 5)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("checkInt mismatch", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkInt("count", 5, 3)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Expected != "5" {
			t.Errorf("Expected = %q, want %q", c.violations[0].Expected, "5")
		}
	})

	t.Run("checkStr match", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkStr("name", "active", "active")
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("checkStr mismatch", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkStr("name", "active", "disabled")
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("checkToggle nil want", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkToggle("rule", nil, true)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0 (unmanaged)", len(c.violations))
		}
	})

	t.Run("checkToggle want enabled but missing", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		enabled := true
		c.checkToggle("rule", &enabled, false)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Expected != "enabled" {
			t.Errorf("Expected = %q, want %q", c.violations[0].Expected, "enabled")
		}
	})

	t.Run("checkToggle want disabled but present", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		disabled := false
		c.checkToggle("rule", &disabled, true)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Expected != "disabled" {
			t.Errorf("Expected = %q, want %q", c.violations[0].Expected, "disabled")
		}
	})

	t.Run("checkToggle matching", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		enabled := true
		c.checkToggle("rule", &enabled, true)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("checkMissing with missing items", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkMissing("envs", []string{"prod", "staging"}, []string{"prod"})
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Message == "" {
			t.Error("expected message with missing items")
		}
	})

	t.Run("checkMissing all present", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.checkMissing("envs", []string{"prod"}, []string{"prod", "staging"})
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "test/"}
		c.absent("rules/creation")
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Field != "test/rules/creation" {
			t.Errorf("Field = %q", c.violations[0].Field)
		}
	})
}

func TestCheckBypassActors(t *testing.T) {
	t.Run("no desired actors", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkBypassActors(nil, nil)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("all present", func(t *testing.T) {
		at := gogithub.BypassActorType("OrganizationAdmin")
		bm := gogithub.BypassMode("always")
		c := &rulesetChecker{prefix: "rs/"}
		c.checkBypassActors(
			[]*gogithub.BypassActor{{ActorID: gogithub.Ptr(int64(1)), ActorType: &at, BypassMode: &bm}},
			[]BypassActorConfig{{ActorType: "OrganizationAdmin", BypassMode: "always"}},
		)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("missing actor", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkBypassActors(
			nil,
			[]BypassActorConfig{{ActorID: 42, ActorType: "Team", BypassMode: "pull_request"}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})
}

func TestCheckConditions(t *testing.T) {
	t.Run("ref name present and matching", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkConditions(
			&gogithub.RepositoryRulesetConditions{
				RefName: &gogithub.RepositoryRulesetRefConditionParameters{Include: []string{"refs/heads/main"}},
			},
			&ConditionsConfig{RefName: &RefConditionConfig{Include: []string{"refs/heads/main"}}},
		)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0: %v", len(c.violations), c.violations)
		}
	})

	t.Run("ref name missing", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkConditions(
			nil,
			&ConditionsConfig{RefName: &RefConditionConfig{Include: []string{"refs/heads/main"}}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("repository name missing", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkConditions(
			nil,
			&ConditionsConfig{RepositoryName: &RepoConditionConfig{Include: []string{"my-repo"}}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("repository name matching", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkConditions(
			&gogithub.RepositoryRulesetConditions{
				RepositoryName: &gogithub.RepositoryRulesetRepositoryNamesConditionParameters{Include: []string{"my-repo"}},
			},
			&ConditionsConfig{RepositoryName: &RepoConditionConfig{Include: []string{"my-repo"}}},
		)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})
}

func TestCheckRules(t *testing.T) {
	t.Run("nil actual treated as empty", func(t *testing.T) {
		enabled := true
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(nil, RulesConfig{Creation: &enabled})
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("toggle rules compliant", func(t *testing.T) {
		enabled := true
		disabled := false
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				Creation:              &gogithub.EmptyRuleParameters{},
				RequiredLinearHistory: &gogithub.EmptyRuleParameters{},
			},
			RulesConfig{
				Creation:             &enabled,
				RequireLinearHistory: &enabled,
				Deletion:             &disabled,
			},
		)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0: %v", len(c.violations), c.violations)
		}
	})

	t.Run("pull request absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{PullRequest: &PullRequestRuleConfig{RequiredApprovals: 2}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/pull-request" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing pull-request rule")
		}
	})

	t.Run("pull request drift", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				PullRequest: &gogithub.PullRequestRuleParameters{
					RequiredApprovingReviewCount: 1,
					DismissStaleReviewsOnPush:    false,
					RequireCodeOwnerReview:       false,
					AllowedMergeMethods:          []gogithub.PullRequestMergeMethod{"merge"},
				},
			},
			RulesConfig{PullRequest: &PullRequestRuleConfig{
				RequiredApprovals:           2,
				DismissStaleApprovalsOnPush: true,
				RequireCodeOwnerReview:      true,
				AllowedMergeMethods:         []string{"squash"},
			}},
		)
		if len(c.violations) < 3 {
			t.Errorf("violations = %d, want at least 3: %v", len(c.violations), c.violations)
		}
	})

	t.Run("required status checks absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{RequiredStatusChecks: &RequiredStatusChecksRuleConfig{
				RequiredStatusChecks: []StatusCheckConfig{{Context: "ci/build"}},
			}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/required-status-checks" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing required-status-checks")
		}
	})

	t.Run("required status checks drift", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		doNotEnforce := false
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				RequiredStatusChecks: &gogithub.RequiredStatusChecksRuleParameters{
					StrictRequiredStatusChecksPolicy: false,
					DoNotEnforceOnCreate:             &doNotEnforce,
					RequiredStatusChecks:             []*gogithub.RuleStatusCheck{{Context: "ci/lint"}},
				},
			},
			RulesConfig{RequiredStatusChecks: &RequiredStatusChecksRuleConfig{
				RequireBranchesToBeUpToDate: true,
				RequiredStatusChecks:        []StatusCheckConfig{{Context: "ci/build"}},
			}},
		)
		if len(c.violations) < 2 {
			t.Errorf("violations = %d, want at least 2: %v", len(c.violations), c.violations)
		}
	})

	t.Run("required deployments", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				RequiredDeployments: &gogithub.RequiredDeploymentsRuleParameters{
					RequiredDeploymentEnvironments: []string{"staging"},
				},
			},
			RulesConfig{RequiredDeployments: &RequiredDeploymentsRuleConfig{
				RequiredDeploymentEnvironments: []string{"staging", "prod"},
			}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("required deployments absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{RequiredDeployments: &RequiredDeploymentsRuleConfig{
				RequiredDeploymentEnvironments: []string{"prod"},
			}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/required-deployments" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing required-deployments")
		}
	})

	t.Run("merge queue absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{MergeQueue: &MergeQueueRuleConfig{MergeMethod: "squash"}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/merge-queue" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing merge-queue")
		}
	})

	t.Run("merge queue drift", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				MergeQueue: &gogithub.MergeQueueRuleParameters{
					MergeMethod:      "merge",
					GroupingStrategy: "ALLGREEN",
				},
			},
			RulesConfig{MergeQueue: &MergeQueueRuleConfig{
				MergeMethod:      "squash",
				GroupingStrategy: "ALLGREEN",
			}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1: %v", len(c.violations), c.violations)
		}
	})

	t.Run("code scanning absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{CodeScanning: &CodeScanningRuleConfig{
				CodeScanningTools: []CodeScanningToolConfig{{Tool: "CodeQL"}},
			}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/code-scanning" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing code-scanning")
		}
	})

	t.Run("code scanning tool missing", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				CodeScanning: &gogithub.CodeScanningRuleParameters{
					CodeScanningTools: []*gogithub.RuleCodeScanningTool{},
				},
			},
			RulesConfig{CodeScanning: &CodeScanningRuleConfig{
				CodeScanningTools: []CodeScanningToolConfig{{Tool: "CodeQL", AlertsThreshold: "errors"}},
			}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("code scanning tool threshold drift", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				CodeScanning: &gogithub.CodeScanningRuleParameters{
					CodeScanningTools: []*gogithub.RuleCodeScanningTool{
						{Tool: "CodeQL", AlertsThreshold: "warnings", SecurityAlertsThreshold: "low"},
					},
				},
			},
			RulesConfig{CodeScanning: &CodeScanningRuleConfig{
				CodeScanningTools: []CodeScanningToolConfig{
					{Tool: "CodeQL", AlertsThreshold: "errors", SecurityAlertsThreshold: "high_or_higher"},
				},
			}},
		)
		if len(c.violations) != 2 {
			t.Fatalf("violations = %d, want 2: %v", len(c.violations), c.violations)
		}
	})

	t.Run("copilot code review absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{CopilotCodeReview: &CopilotCodeReviewRuleConfig{ReviewOnPush: true}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/copilot-code-review" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing copilot-code-review")
		}
	})

	t.Run("copilot code review drift", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				CopilotCodeReview: &gogithub.CopilotCodeReviewRuleParameters{
					ReviewOnPush:            false,
					ReviewDraftPullRequests: false,
				},
			},
			RulesConfig{CopilotCodeReview: &CopilotCodeReviewRuleConfig{
				ReviewOnPush:            true,
				ReviewDraftPullRequests: true,
			}},
		)
		if len(c.violations) != 2 {
			t.Fatalf("violations = %d, want 2: %v", len(c.violations), c.violations)
		}
	})

	t.Run("commit message pattern absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{CommitMessagePattern: &PatternRuleConfig{Operator: "starts_with", Pattern: "feat"}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/commit-message-pattern" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing commit-message-pattern")
		}
	})

	t.Run("branch name pattern absent", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{},
			RulesConfig{BranchNamePattern: &PatternRuleConfig{Operator: "regex", Pattern: "^[a-z]"}},
		)
		found := false
		for _, v := range c.violations {
			if v.Field == "rs/rules/branch-name-pattern" {
				found = true
			}
		}
		if !found {
			t.Error("expected violation for missing branch-name-pattern")
		}
	})

	t.Run("branch name pattern drift", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				BranchNamePattern: &gogithub.PatternRuleParameters{
					Operator: "regex", Pattern: "^[a-z]", Negate: gogithub.Ptr(false),
				},
			},
			RulesConfig{BranchNamePattern: &PatternRuleConfig{
				Operator: "regex", Pattern: "^[a-z-]+$", Negate: false,
			}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1: %v", len(c.violations), c.violations)
		}
	})

	t.Run("pattern rule compliant", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRules(
			&gogithub.RepositoryRulesetRules{
				CommitMessagePattern: &gogithub.PatternRuleParameters{
					Operator: "starts_with", Pattern: "feat", Negate: gogithub.Ptr(false),
				},
			},
			RulesConfig{CommitMessagePattern: &PatternRuleConfig{
				Operator: "starts_with", Pattern: "feat", Negate: false,
			}},
		)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0: %v", len(c.violations), c.violations)
		}
	})
}

func TestCheckRequiredReviewers(t *testing.T) {
	t.Run("no desired reviewers", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRequiredReviewers(nil, nil)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0", len(c.violations))
		}
	})

	t.Run("matching team", func(t *testing.T) {
		minApprovals := 2
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRequiredReviewers(
			[]RequiredReviewerConfig{{TeamID: 10, MinimumApprovals: 2}},
			[]*gogithub.RulesetRequiredReviewer{{
				MinimumApprovals: &minApprovals,
				Reviewer:         &gogithub.RulesetReviewer{ID: gogithub.Ptr(int64(10))},
			}},
		)
		if len(c.violations) != 0 {
			t.Errorf("violations = %d, want 0: %v", len(c.violations), c.violations)
		}
	})

	t.Run("missing team", func(t *testing.T) {
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRequiredReviewers(
			[]RequiredReviewerConfig{{TeamID: 10, MinimumApprovals: 1}},
			nil,
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
	})

	t.Run("unexpected team", func(t *testing.T) {
		minApprovals := 1
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRequiredReviewers(
			nil,
			[]*gogithub.RulesetRequiredReviewer{{
				MinimumApprovals: &minApprovals,
				Reviewer:         &gogithub.RulesetReviewer{ID: gogithub.Ptr(int64(99))},
			}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(c.violations))
		}
		if c.violations[0].Expected != "absent" {
			t.Errorf("Expected = %q, want %q", c.violations[0].Expected, "absent")
		}
	})

	t.Run("file patterns checked", func(t *testing.T) {
		minApprovals := 1
		c := &rulesetChecker{prefix: "rs/"}
		c.checkRequiredReviewers(
			[]RequiredReviewerConfig{{TeamID: 10, MinimumApprovals: 1, FilePatterns: []string{"*.go", "*.ts"}}},
			[]*gogithub.RulesetRequiredReviewer{{
				MinimumApprovals: &minApprovals,
				FilePatterns:     []string{"*.go"},
				Reviewer:         &gogithub.RulesetReviewer{ID: gogithub.Ptr(int64(10))},
			}},
		)
		if len(c.violations) != 1 {
			t.Fatalf("violations = %d, want 1 (missing *.ts)", len(c.violations))
		}
	})
}

func TestCheckRuleset(t *testing.T) {
	t.Run("fully compliant", func(t *testing.T) {
		creation := true
		actual := &gogithub.RepositoryRuleset{
			Enforcement: "active",
			Rules: &gogithub.RepositoryRulesetRules{
				Creation: &gogithub.EmptyRuleParameters{},
			},
		}
		violations := checkRuleset("main", actual, RulesetConfig{
			Enforcement: "active",
			Rules:       RulesConfig{Creation: &creation},
		})
		if len(violations) != 0 {
			t.Errorf("violations = %d, want 0: %v", len(violations), violations)
		}
	})

	t.Run("with conditions checked", func(t *testing.T) {
		actual := &gogithub.RepositoryRuleset{
			Enforcement: "active",
			Conditions: &gogithub.RepositoryRulesetConditions{
				RefName: &gogithub.RepositoryRulesetRefConditionParameters{Include: []string{"refs/heads/main"}},
			},
			Rules: &gogithub.RepositoryRulesetRules{},
		}
		violations := checkRuleset("main", actual, RulesetConfig{
			Enforcement: "active",
			Conditions: &ConditionsConfig{
				RefName: &RefConditionConfig{Include: []string{"refs/heads/main"}},
			},
		})
		if len(violations) != 0 {
			t.Errorf("violations = %d, want 0: %v", len(violations), violations)
		}
	})

	t.Run("enforcement drift", func(t *testing.T) {
		actual := &gogithub.RepositoryRuleset{
			Enforcement: "disabled",
			Rules:       &gogithub.RepositoryRulesetRules{},
		}
		violations := checkRuleset("main", actual, RulesetConfig{Enforcement: "active"})
		found := false
		for _, v := range violations {
			if v.Field == "main/enforcement" {
				found = true
			}
		}
		if !found {
			t.Error("expected enforcement violation")
		}
	})
}
