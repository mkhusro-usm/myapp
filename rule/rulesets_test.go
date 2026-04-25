package rule

import (
	"errors"
	"testing"

	gogithub "github.com/google/go-github/v84/github"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

func TestRepoRulesetsName(t *testing.T) {
	r := NewRepoRulesets(&stubRulesetsClient{}, RulesetsSettings{})
	if r.Name() != "repo-rulesets" {
		t.Errorf("Name() = %q, want %q", r.Name(), "repo-rulesets")
	}
}

func TestWithResolvedName(t *testing.T) {
	cfg := RulesetConfig{Name: "{default_branch}-protection"}
	resolved := cfg.withResolvedName("main")
	if resolved.Name != "main-protection" {
		t.Errorf("Name = %q, want %q", resolved.Name, "main-protection")
	}
	if cfg.Name != "{default_branch}-protection" {
		t.Error("original should be unmodified")
	}
}

func TestBuildRuleset(t *testing.T) {
	t.Run("basic fields", func(t *testing.T) {
		creation := true
		deletion := true
		blockForce := true
		cfg := RulesetConfig{
			Name:        "main-protection",
			Target:      "branch",
			Enforcement: "active",
			Rules: RulesConfig{
				Creation:         &creation,
				Deletion:         &deletion,
				BlockForcePushes: &blockForce,
			},
		}
		rs := buildRuleset(cfg)
		if rs.Name != "main-protection" {
			t.Errorf("Name = %q, want %q", rs.Name, "main-protection")
		}
		if rs.Enforcement != "active" {
			t.Errorf("Enforcement = %q, want %q", rs.Enforcement, "active")
		}
		if rs.Rules.Creation == nil {
			t.Error("expected Creation rule")
		}
		if rs.Rules.Deletion == nil {
			t.Error("expected Deletion rule")
		}
		if rs.Rules.NonFastForward == nil {
			t.Error("expected NonFastForward rule")
		}
	})

	t.Run("bypass actors", func(t *testing.T) {
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			BypassActors: []BypassActorConfig{
				{ActorType: "OrganizationAdmin", BypassMode: "always"},
				{ActorID: 42, ActorType: "Team", BypassMode: "pull_request"},
			},
		}
		rs := buildRuleset(cfg)
		if len(rs.BypassActors) != 2 {
			t.Fatalf("bypass actors count = %d, want 2", len(rs.BypassActors))
		}
		if *rs.BypassActors[0].ActorID != 1 {
			t.Errorf("OrgAdmin ActorID = %d, want 1 (auto-assigned)", *rs.BypassActors[0].ActorID)
		}
		if *rs.BypassActors[1].ActorID != 42 {
			t.Errorf("Team ActorID = %d, want 42", *rs.BypassActors[1].ActorID)
		}
	})

	t.Run("conditions", func(t *testing.T) {
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			Conditions: &ConditionsConfig{
				RefName: &RefConditionConfig{
					Include: []string{"refs/heads/main"},
					Exclude: []string{"refs/heads/dev"},
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Conditions == nil || rs.Conditions.RefName == nil {
			t.Fatal("expected conditions with ref name")
		}
		if rs.Conditions.RefName.Include[0] != "refs/heads/main" {
			t.Errorf("Include[0] = %q, want refs/heads/main", rs.Conditions.RefName.Include[0])
		}
	})

	t.Run("pull request rule", func(t *testing.T) {
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			Rules: RulesConfig{
				PullRequest: &PullRequestRuleConfig{
					RequiredApprovals:             2,
					DismissStaleApprovalsOnPush:   true,
					RequireCodeOwnerReview:        true,
					RequireMostRecentPushApproval: false,
					AllowedMergeMethods:           []string{"squash", "merge"},
				},
			},
		}
		rs := buildRuleset(cfg)
		pr := rs.Rules.PullRequest
		if pr == nil {
			t.Fatal("expected PullRequest rule")
		}
		if pr.RequiredApprovingReviewCount != 2 {
			t.Errorf("RequiredApprovingReviewCount = %d, want 2", pr.RequiredApprovingReviewCount)
		}
		if !pr.DismissStaleReviewsOnPush {
			t.Error("expected DismissStaleReviewsOnPush = true")
		}
		if len(pr.AllowedMergeMethods) != 2 {
			t.Errorf("AllowedMergeMethods count = %d, want 2", len(pr.AllowedMergeMethods))
		}
	})

	t.Run("required status checks skipped when empty", func(t *testing.T) {
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			Rules: RulesConfig{
				RequiredStatusChecks: &RequiredStatusChecksRuleConfig{
					RequireBranchesToBeUpToDate: true,
					RequiredStatusChecks:        []StatusCheckConfig{},
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.RequiredStatusChecks != nil {
			t.Error("expected nil RequiredStatusChecks when checks list is empty")
		}
	})

	t.Run("required status checks populated", func(t *testing.T) {
		intID := int64(123)
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			Rules: RulesConfig{
				RequiredStatusChecks: &RequiredStatusChecksRuleConfig{
					RequireBranchesToBeUpToDate:      true,
					DoNotRequireStatusChecksOnCreate: true,
					RequiredStatusChecks: []StatusCheckConfig{
						{Context: "ci/build", IntegrationID: &intID},
					},
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.RequiredStatusChecks == nil {
			t.Fatal("expected RequiredStatusChecks")
		}
		if len(rs.Rules.RequiredStatusChecks.RequiredStatusChecks) != 1 {
			t.Errorf("checks count = %d, want 1", len(rs.Rules.RequiredStatusChecks.RequiredStatusChecks))
		}
	})

	t.Run("code scanning", func(t *testing.T) {
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			Rules: RulesConfig{
				CodeScanning: &CodeScanningRuleConfig{
					CodeScanningTools: []CodeScanningToolConfig{
						{Tool: "CodeQL", AlertsThreshold: "errors", SecurityAlertsThreshold: "high_or_higher"},
					},
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.CodeScanning == nil {
			t.Fatal("expected CodeScanning rule")
		}
		if len(rs.Rules.CodeScanning.CodeScanningTools) != 1 {
			t.Errorf("tools count = %d, want 1", len(rs.Rules.CodeScanning.CodeScanningTools))
		}
	})

	t.Run("all toggle rules", func(t *testing.T) {
		enabled := true
		cfg := RulesetConfig{
			Name: "test", Target: "branch", Enforcement: "active",
			Rules: RulesConfig{
				Creation:             &enabled,
				Update:               &enabled,
				Deletion:             &enabled,
				RequireLinearHistory: &enabled,
				RequireSignedCommits: &enabled,
				BlockForcePushes:     &enabled,
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.Update == nil {
			t.Error("expected Update rule")
		}
		if rs.Rules.RequiredLinearHistory == nil {
			t.Error("expected RequiredLinearHistory rule")
		}
		if rs.Rules.RequiredSignatures == nil {
			t.Error("expected RequiredSignatures rule")
		}
	})

	t.Run("required deployments", func(t *testing.T) {
		cfg := RulesetConfig{
			Name: "test", Target: "branch", Enforcement: "active",
			Rules: RulesConfig{
				RequiredDeployments: &RequiredDeploymentsRuleConfig{
					RequiredDeploymentEnvironments: []string{"staging", "prod"},
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.RequiredDeployments == nil {
			t.Fatal("expected RequiredDeployments rule")
		}
		if len(rs.Rules.RequiredDeployments.RequiredDeploymentEnvironments) != 2 {
			t.Errorf("envs count = %d, want 2", len(rs.Rules.RequiredDeployments.RequiredDeploymentEnvironments))
		}
	})

	t.Run("pull request with required reviewers", func(t *testing.T) {
		cfg := RulesetConfig{
			Name: "test", Target: "branch", Enforcement: "active",
			Rules: RulesConfig{
				PullRequest: &PullRequestRuleConfig{
					RequiredApprovals: 1,
					RequiredReviewers: []RequiredReviewerConfig{
						{TeamID: 42, MinimumApprovals: 2, FilePatterns: []string{"*.go"}},
					},
				},
			},
		}
		rs := buildRuleset(cfg)
		if len(rs.Rules.PullRequest.RequiredReviewers) != 1 {
			t.Fatalf("required reviewers = %d, want 1", len(rs.Rules.PullRequest.RequiredReviewers))
		}
		rr := rs.Rules.PullRequest.RequiredReviewers[0]
		if *rr.Reviewer.ID != 42 {
			t.Errorf("Reviewer.ID = %d, want 42", *rr.Reviewer.ID)
		}
		if *rr.MinimumApprovals != 2 {
			t.Errorf("MinimumApprovals = %d, want 2", *rr.MinimumApprovals)
		}
	})

	t.Run("copilot code review", func(t *testing.T) {
		cfg := RulesetConfig{
			Name: "test", Target: "branch", Enforcement: "active",
			Rules: RulesConfig{
				CopilotCodeReview: &CopilotCodeReviewRuleConfig{
					ReviewOnPush: true, ReviewDraftPullRequests: false,
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.CopilotCodeReview == nil {
			t.Fatal("expected CopilotCodeReview rule")
		}
		if !rs.Rules.CopilotCodeReview.ReviewOnPush {
			t.Error("expected ReviewOnPush = true")
		}
	})

	t.Run("merge queue", func(t *testing.T) {
		cfg := RulesetConfig{
			Name: "test", Target: "branch", Enforcement: "active",
			Rules: RulesConfig{
				MergeQueue: &MergeQueueRuleConfig{
					MergeMethod:       "squash",
					GroupingStrategy:  "ALLGREEN",
					MaxEntriesToBuild: 5,
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.MergeQueue == nil {
			t.Fatal("expected MergeQueue rule")
		}
		if string(rs.Rules.MergeQueue.MergeMethod) != "squash" {
			t.Errorf("MergeMethod = %q, want squash", rs.Rules.MergeQueue.MergeMethod)
		}
	})

	t.Run("repository name condition", func(t *testing.T) {
		cfg := RulesetConfig{
			Name: "test", Target: "branch", Enforcement: "active",
			Conditions: &ConditionsConfig{
				RepositoryName: &RepoConditionConfig{
					Include:   []string{"my-*"},
					Exclude:   []string{"my-internal"},
					Protected: gogithub.Ptr(true),
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Conditions == nil || rs.Conditions.RepositoryName == nil {
			t.Fatal("expected RepositoryName condition")
		}
		if rs.Conditions.RepositoryName.Include[0] != "my-*" {
			t.Errorf("Include[0] = %q, want my-*", rs.Conditions.RepositoryName.Include[0])
		}
	})

	t.Run("pattern rules", func(t *testing.T) {
		cfg := RulesetConfig{
			Name:        "test",
			Target:      "branch",
			Enforcement: "active",
			Rules: RulesConfig{
				CommitMessagePattern: &PatternRuleConfig{
					Name: "conventional", Operator: "starts_with", Pattern: "feat|fix", Negate: false,
				},
				BranchNamePattern: &PatternRuleConfig{
					Name: "kebab", Operator: "regex", Pattern: "^[a-z-]+$", Negate: false,
				},
			},
		}
		rs := buildRuleset(cfg)
		if rs.Rules.CommitMessagePattern == nil {
			t.Error("expected CommitMessagePattern")
		}
		if rs.Rules.BranchNamePattern == nil {
			t.Error("expected BranchNamePattern")
		}
	})
}

func TestBuildPatternRule(t *testing.T) {
	cfg := &PatternRuleConfig{
		Name: "test-pattern", Operator: "starts_with", Pattern: "feat/", Negate: true,
	}
	pr := buildPatternRule(cfg)
	if *pr.Name != "test-pattern" {
		t.Errorf("Name = %q, want %q", *pr.Name, "test-pattern")
	}
	if *pr.Negate != true {
		t.Error("expected Negate = true")
	}
	if pr.Pattern != "feat/" {
		t.Errorf("Pattern = %q, want %q", pr.Pattern, "feat/")
	}
}

func TestRepoRulesetsEvaluate(t *testing.T) {
	t.Run("compliant", func(t *testing.T) {
		enforcement := gogithub.RulesetEnforcement("active")
		stub := &stubRulesetsClient{
			ruleset: &gogithub.RepositoryRuleset{
				ID:          gogithub.Ptr(int64(1)),
				Name:        "main-protection",
				Enforcement: enforcement,
				Rules:       &gogithub.RepositoryRulesetRules{},
			},
		}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{
				Name: "main-protection", Enforcement: "active",
			}},
		})

		result, err := r.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compliant {
			t.Errorf("expected compliant, got violations: %v", result.Violations)
		}
	})

	t.Run("ruleset not found", func(t *testing.T) {
		stub := &stubRulesetsClient{ruleset: nil}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "missing"}},
		})

		result, err := r.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compliant {
			t.Error("expected non-compliant when ruleset missing")
		}
	})

	t.Run("client error", func(t *testing.T) {
		stub := &stubRulesetsClient{getErr: errors.New("api error")}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "main"}},
		})

		_, err := r.Evaluate(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRepoRulesetsApply(t *testing.T) {
	t.Run("creates when not found", func(t *testing.T) {
		stub := &stubRulesetsClient{
			ruleset: nil,
			created: &gogithub.RepositoryRuleset{ID: gogithub.Ptr(int64(1))},
		}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "main", Target: "branch", Enforcement: "active"}},
		})

		result, err := r.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Applied {
			t.Error("expected Applied = true")
		}
	})

	t.Run("updates when found", func(t *testing.T) {
		stub := &stubRulesetsClient{
			ruleset: &gogithub.RepositoryRuleset{ID: gogithub.Ptr(int64(99))},
			updated: &gogithub.RepositoryRuleset{ID: gogithub.Ptr(int64(99))},
		}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "main", Target: "branch", Enforcement: "active"}},
		})

		result, err := r.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Applied {
			t.Error("expected Applied = true")
		}
	})

	t.Run("get error", func(t *testing.T) {
		stub := &stubRulesetsClient{getErr: errors.New("api error")}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "main"}},
		})

		_, err := r.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create error", func(t *testing.T) {
		stub := &stubRulesetsClient{
			ruleset:   nil,
			createErr: errors.New("422"),
		}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "main", Target: "branch", Enforcement: "active"}},
		})

		_, err := r.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update error", func(t *testing.T) {
		stub := &stubRulesetsClient{
			ruleset:   &gogithub.RepositoryRuleset{ID: gogithub.Ptr(int64(99))},
			updateErr: errors.New("500"),
		}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "main", Target: "branch", Enforcement: "active"}},
		})

		_, err := r.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "main"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolves default branch placeholder", func(t *testing.T) {
		stub := &stubRulesetsClient{
			ruleset: nil,
			created: &gogithub.RepositoryRuleset{ID: gogithub.Ptr(int64(1))},
		}
		r := NewRepoRulesets(stub, RulesetsSettings{
			Rulesets: []RulesetConfig{{Name: "{default_branch}-protection", Target: "branch", Enforcement: "active"}},
		})

		result, err := r.Apply(t.Context(), &gh.Repository{Name: "repo-a", Owner: "org", DefaultBranch: "develop"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Applied {
			t.Error("expected Applied = true")
		}
	})
}
