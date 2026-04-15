package github

import (
	"context"
	"fmt"
	"path"

	"github.com/shurcooL/githubv4"
)

// BranchProtectionRule represents the current state of a branch protection rule.
type BranchProtectionRule struct {
	ID                             string
	Pattern                        string
	RequiresApprovingReviews       bool
	RequiredApprovingReviewCount   int
	DismissesStaleReviews          bool
	RequiresCodeOwnerReviews       bool
	RequireLastPushApproval        bool
	RequiresStatusChecks           bool
	RequiresStrictStatusChecks     bool
	RequiredStatusCheckContexts    []string
	RequiresDeployments            bool
	RequiredDeploymentEnvironments []string
	IsAdminEnforced                bool
	RequiresLinearHistory          bool
	RequiresCommitSignatures       bool
	RequiresConversationResolution bool
	AllowsForcePushes              bool
	AllowsDeletions                bool
	BlocksCreations                bool
	LockBranch                     bool
	LockAllowsFetchAndMerge        bool
	RestrictsPushes                bool
	RestrictsReviewDismissals      bool
	BypassPullRequestActors        []Actor
	PushAllowanceActors            []Actor
	ReviewDismissalActors          []Actor
}

// BranchProtectionInput represents the desired state to apply for a branch protection rule.
// Pointer fields are optional — only non-nil fields are included in the GraphQL mutation.
type BranchProtectionInput struct {
	RequiresApprovingReviews       *bool
	RequiredApprovingReviewCount   *int
	DismissesStaleReviews          *bool
	RequiresCodeOwnerReviews       *bool
	RequireLastPushApproval        *bool
	RequiresStatusChecks           *bool
	RequiresStrictStatusChecks     *bool
	RequiredStatusCheckContexts    []string
	RequiresDeployments            *bool
	RequiredDeploymentEnvironments []string
	IsAdminEnforced                *bool
	RequiresLinearHistory          *bool
	RequiresCommitSignatures       *bool
	RequiresConversationResolution *bool
	AllowsForcePushes              *bool
	AllowsDeletions                *bool
	BlocksCreations                *bool
	LockBranch                     *bool
	LockAllowsFetchAndMerge        *bool
	RestrictsPushes                *bool
	RestrictsReviewDismissals      *bool
	BypassPullRequestActorIDs      []string
	PushActorIDs                   []string
	ReviewDismissalActorIDs        []string
}

// GetBranchProtectionRule fetches the branch protection rule matching the given branch.
// Returns nil (without error) if no rule matches.
func (c *Client) GetBranchProtectionRule(ctx context.Context, repoName, branch string) (*BranchProtectionRule, error) {
	var q struct {
		Repository struct {
			BranchProtectionRules struct {
				Nodes []struct {
					ID                             githubv4.ID
					Pattern                        string
					RequiresApprovingReviews       bool
					RequiredApprovingReviewCount   int
					DismissesStaleReviews          bool
					RequiresCodeOwnerReviews       bool
					RequireLastPushApproval        bool
					RequiresStatusChecks           bool
					RequiresStrictStatusChecks     bool
					RequiredStatusCheckContexts    []string
					RequiresDeployments            bool
					RequiredDeploymentEnvironments []string
					IsAdminEnforced                bool
					RequiresLinearHistory          bool
					RequiresCommitSignatures       bool
					RequiresConversationResolution bool
					AllowsForcePushes              bool
					AllowsDeletions                bool
					BlocksCreations                bool
					LockBranch                     bool
					LockAllowsFetchAndMerge        bool
					RestrictsPushes                bool
					RestrictsReviewDismissals      bool
					BypassPullRequestAllowances    struct {
						Nodes []actorNode
					} `graphql:"bypassPullRequestAllowances(first: 100)"`
					PushAllowances struct {
						Nodes []actorNode
					} `graphql:"pushAllowances(first: 100)"`
					ReviewDismissalAllowances struct {
						Nodes []actorNode
					} `graphql:"reviewDismissalAllowances(first: 100)"`
				}
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"branchProtectionRules(first: 100, after: $cursor)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":  githubv4.String(c.org),
		"name":   githubv4.String(repoName),
		"cursor": (*githubv4.String)(nil),
	}

	for {
		if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("querying branch protection rules for %s: %w", repoName, err)
		}
		for _, n := range q.Repository.BranchProtectionRules.Nodes {
			matched, err := path.Match(n.Pattern, branch)
			if err != nil || !matched {
				continue
			}
			return &BranchProtectionRule{
				ID:                             n.ID.(string),
				Pattern:                        n.Pattern,
				RequiresApprovingReviews:       n.RequiresApprovingReviews,
				RequiredApprovingReviewCount:   n.RequiredApprovingReviewCount,
				DismissesStaleReviews:          n.DismissesStaleReviews,
				RequiresCodeOwnerReviews:       n.RequiresCodeOwnerReviews,
				RequireLastPushApproval:        n.RequireLastPushApproval,
				RequiresStatusChecks:           n.RequiresStatusChecks,
				RequiresStrictStatusChecks:     n.RequiresStrictStatusChecks,
				RequiredStatusCheckContexts:    n.RequiredStatusCheckContexts,
				RequiresDeployments:            n.RequiresDeployments,
				RequiredDeploymentEnvironments: n.RequiredDeploymentEnvironments,
				IsAdminEnforced:                n.IsAdminEnforced,
				RequiresLinearHistory:          n.RequiresLinearHistory,
				RequiresCommitSignatures:       n.RequiresCommitSignatures,
				RequiresConversationResolution: n.RequiresConversationResolution,
				AllowsForcePushes:              n.AllowsForcePushes,
				AllowsDeletions:                n.AllowsDeletions,
				BlocksCreations:                n.BlocksCreations,
				LockBranch:                     n.LockBranch,
				LockAllowsFetchAndMerge:        n.LockAllowsFetchAndMerge,
				RestrictsPushes:                n.RestrictsPushes,
				RestrictsReviewDismissals:      n.RestrictsReviewDismissals,
				BypassPullRequestActors:        parseActors(n.BypassPullRequestAllowances.Nodes),
				PushAllowanceActors:            parseActors(n.PushAllowances.Nodes),
				ReviewDismissalActors:          parseActors(n.ReviewDismissalAllowances.Nodes),
			}, nil
		}
		if !q.Repository.BranchProtectionRules.PageInfo.HasNextPage {
			break
		}
		variables["cursor"] = githubv4.NewString(q.Repository.BranchProtectionRules.PageInfo.EndCursor)
	}

	return nil, nil
}

// CreateBranchProtectionRule creates a new branch protection rule via GraphQL.
func (c *Client) CreateBranchProtectionRule(ctx context.Context, repoID, pattern string, input BranchProtectionInput) (string, error) {
	var m struct {
		CreateBranchProtectionRule struct {
			BranchProtectionRule struct {
				ID githubv4.ID
			}
		} `graphql:"createBranchProtectionRule(input: $input)"`
	}

	gqlInput := githubv4.CreateBranchProtectionRuleInput{
		RepositoryID:                   repoID,
		Pattern:                        githubv4.String(pattern),
		RequiresApprovingReviews:       gqlBool(input.RequiresApprovingReviews),
		RequiredApprovingReviewCount:   gqlInt(input.RequiredApprovingReviewCount),
		DismissesStaleReviews:          gqlBool(input.DismissesStaleReviews),
		RequiresCodeOwnerReviews:       gqlBool(input.RequiresCodeOwnerReviews),
		RequireLastPushApproval:        gqlBool(input.RequireLastPushApproval),
		RequiresStatusChecks:           gqlBool(input.RequiresStatusChecks),
		RequiresStrictStatusChecks:     gqlBool(input.RequiresStrictStatusChecks),
		RequiredStatusCheckContexts:    gqlStringsPtr(input.RequiredStatusCheckContexts),
		RequiresDeployments:            gqlBool(input.RequiresDeployments),
		RequiredDeploymentEnvironments: gqlStringsPtr(input.RequiredDeploymentEnvironments),
		IsAdminEnforced:                gqlBool(input.IsAdminEnforced),
		RequiresLinearHistory:          gqlBool(input.RequiresLinearHistory),
		RequiresCommitSignatures:       gqlBool(input.RequiresCommitSignatures),
		RequiresConversationResolution: gqlBool(input.RequiresConversationResolution),
		AllowsForcePushes:              gqlBool(input.AllowsForcePushes),
		AllowsDeletions:                gqlBool(input.AllowsDeletions),
		BlocksCreations:                gqlBool(input.BlocksCreations),
		LockBranch:                     gqlBool(input.LockBranch),
		LockAllowsFetchAndMerge:        gqlBool(input.LockAllowsFetchAndMerge),
		RestrictsPushes:                gqlBool(input.RestrictsPushes),
		RestrictsReviewDismissals:      gqlBool(input.RestrictsReviewDismissals),
		BypassPullRequestActorIDs:      gqlIDsPtr(input.BypassPullRequestActorIDs),
		PushActorIDs:                   gqlIDsPtr(input.PushActorIDs),
		ReviewDismissalActorIDs:        gqlIDsPtr(input.ReviewDismissalActorIDs),
	}

	if err := c.GraphQL.Mutate(ctx, &m, gqlInput, nil); err != nil {
		return "", fmt.Errorf("creating branch protection rule for pattern %q: %w", pattern, err)
	}

	return m.CreateBranchProtectionRule.BranchProtectionRule.ID.(string), nil
}

// UpdateBranchProtectionRule updates an existing branch protection rule via GraphQL.
func (c *Client) UpdateBranchProtectionRule(ctx context.Context, ruleID string, input BranchProtectionInput) error {
	var m struct {
		UpdateBranchProtectionRule struct {
			BranchProtectionRule struct {
				ID githubv4.ID
			}
		} `graphql:"updateBranchProtectionRule(input: $input)"`
	}

	gqlInput := githubv4.UpdateBranchProtectionRuleInput{
		BranchProtectionRuleID:         ruleID,
		RequiresApprovingReviews:       gqlBool(input.RequiresApprovingReviews),
		RequiredApprovingReviewCount:   gqlInt(input.RequiredApprovingReviewCount),
		DismissesStaleReviews:          gqlBool(input.DismissesStaleReviews),
		RequiresCodeOwnerReviews:       gqlBool(input.RequiresCodeOwnerReviews),
		RequireLastPushApproval:        gqlBool(input.RequireLastPushApproval),
		RequiresStatusChecks:           gqlBool(input.RequiresStatusChecks),
		RequiresStrictStatusChecks:     gqlBool(input.RequiresStrictStatusChecks),
		RequiredStatusCheckContexts:    gqlStringsPtr(input.RequiredStatusCheckContexts),
		RequiresDeployments:            gqlBool(input.RequiresDeployments),
		RequiredDeploymentEnvironments: gqlStringsPtr(input.RequiredDeploymentEnvironments),
		IsAdminEnforced:                gqlBool(input.IsAdminEnforced),
		RequiresLinearHistory:          gqlBool(input.RequiresLinearHistory),
		RequiresCommitSignatures:       gqlBool(input.RequiresCommitSignatures),
		RequiresConversationResolution: gqlBool(input.RequiresConversationResolution),
		AllowsForcePushes:              gqlBool(input.AllowsForcePushes),
		AllowsDeletions:                gqlBool(input.AllowsDeletions),
		BlocksCreations:                gqlBool(input.BlocksCreations),
		LockBranch:                     gqlBool(input.LockBranch),
		LockAllowsFetchAndMerge:        gqlBool(input.LockAllowsFetchAndMerge),
		RestrictsPushes:                gqlBool(input.RestrictsPushes),
		RestrictsReviewDismissals:      gqlBool(input.RestrictsReviewDismissals),
		BypassPullRequestActorIDs:      gqlIDsPtr(input.BypassPullRequestActorIDs),
		PushActorIDs:                   gqlIDsPtr(input.PushActorIDs),
		ReviewDismissalActorIDs:        gqlIDsPtr(input.ReviewDismissalActorIDs),
	}

	if err := c.GraphQL.Mutate(ctx, &m, gqlInput, nil); err != nil {
		return fmt.Errorf("updating branch protection rule %s: %w", ruleID, err)
	}

	return nil
}
