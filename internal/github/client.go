package github

import (
	"context"
	"fmt"
	"net/http"
	"path"

	gogithub "github.com/google/go-github/v62/github"
	"github.com/shurcooL/githubv4"
)

// Repository represents a GitHub repository with fields relevant to governance.
type Repository struct {
	ID            string
	Name          string
	Owner         string
	DefaultBranch string
	IsArchived    bool
	IsFork        bool
}

func (r Repository) FullName() string {
	return r.Owner + "/" + r.Name
}

// BranchProtectionRule represents the current state of a branch protection rule.
type BranchProtectionRule struct {
	ID                           string
	Pattern                      string
	RequiresApprovingReviews     bool
	RequiredApprovingReviewCount int
	RequiresStatusChecks         bool
	RequiresStrictStatusChecks   bool
	IsAdminEnforced              bool
	RequiresLinearHistory        bool
	AllowsForcePushes            bool
	AllowsDeletions              bool
	DismissesStaleReviews        bool
	RequiresCodeOwnerReviews     bool
}

// BranchProtectionInput represents the desired state to apply for a branch protection rule.
type BranchProtectionInput struct {
	RequiresApprovingReviews     bool
	RequiredApprovingReviewCount int
	RequiresStatusChecks         bool
	RequiresStrictStatusChecks   bool
	IsAdminEnforced              bool
	RequiresLinearHistory        bool
	AllowsForcePushes            bool
	AllowsDeletions              bool
}

// Client wraps both the GitHub REST and GraphQL APIs.
// GraphQL is the primary API; REST is available as a fallback.
type Client struct {
	REST    *gogithub.Client
	GraphQL *githubv4.Client
	org     string
}

// NewClient creates a Client wrapping both REST and GraphQL APIs.
func NewClient(httpClient *http.Client, org string) *Client {
	return &Client{
		REST:    gogithub.NewClient(httpClient),
		GraphQL: githubv4.NewClient(httpClient),
		org:     org,
	}
}

func (c *Client) Org() string {
	return c.org
}

// GetRepository fetches a single repository by name via GraphQL.
func (c *Client) GetRepository(ctx context.Context, name string) (*Repository, error) {
	var q struct {
		Repository struct {
			ID               githubv4.ID
			Name             string
			DefaultBranchRef struct {
				Name string
			}
			IsArchived bool
			IsFork     bool
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(c.org),
		"name":  githubv4.String(name),
	}

	if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
		return nil, fmt.Errorf("querying repository %s: %w", name, err)
	}

	return &Repository{
		ID:            q.Repository.ID.(string),
		Name:          q.Repository.Name,
		Owner:         c.org,
		DefaultBranch: q.Repository.DefaultBranchRef.Name,
		IsArchived:    q.Repository.IsArchived,
		IsFork:        q.Repository.IsFork,
	}, nil
}

// ListRepositories fetches all repositories in the organization via GraphQL.
func (c *Client) ListRepositories(ctx context.Context) ([]Repository, error) {
	var q struct {
		Organization struct {
			Repositories struct {
				Nodes []struct {
					ID               githubv4.ID
					Name             string
					DefaultBranchRef struct {
						Name string
					}
					IsArchived bool
					IsFork     bool
				}
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"repositories(first: 100, after: $cursor)"`
		} `graphql:"organization(login: $org)"`
	}

	variables := map[string]interface{}{
		"org":    githubv4.String(c.org),
		"cursor": (*githubv4.String)(nil),
	}

	var repos []Repository
	for {
		if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("querying repositories: %w", err)
		}
		for _, n := range q.Organization.Repositories.Nodes {
			repos = append(repos, Repository{
				ID:            n.ID.(string),
				Name:          n.Name,
				Owner:         c.org,
				DefaultBranch: n.DefaultBranchRef.Name,
				IsArchived:    n.IsArchived,
				IsFork:        n.IsFork,
			})
		}
		if !q.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["cursor"] = githubv4.NewString(q.Organization.Repositories.PageInfo.EndCursor)
	}
	return repos, nil
}

// GetBranchProtectionRules fetches all branch protection rules for a repository via GraphQL.
func (c *Client) GetBranchProtectionRules(ctx context.Context, repoName string) ([]BranchProtectionRule, error) {
	var q struct {
		Repository struct {
			BranchProtectionRules struct {
				Nodes []struct {
					ID                           githubv4.ID
					Pattern                      string
					RequiresApprovingReviews     bool
					RequiredApprovingReviewCount int
					RequiresStatusChecks         bool
					RequiresStrictStatusChecks   bool
					IsAdminEnforced              bool
					RequiresLinearHistory        bool
					AllowsForcePushes            bool
					AllowsDeletions              bool
					DismissesStaleReviews        bool
					RequiresCodeOwnerReviews     bool
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

	var rules []BranchProtectionRule
	for {
		if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("querying branch protection rules for %s: %w", repoName, err)
		}
		for _, n := range q.Repository.BranchProtectionRules.Nodes {
			rules = append(rules, BranchProtectionRule{
				ID:                           n.ID.(string),
				Pattern:                      n.Pattern,
				RequiresApprovingReviews:     n.RequiresApprovingReviews,
				RequiredApprovingReviewCount: n.RequiredApprovingReviewCount,
				RequiresStatusChecks:         n.RequiresStatusChecks,
				RequiresStrictStatusChecks:   n.RequiresStrictStatusChecks,
				IsAdminEnforced:              n.IsAdminEnforced,
				RequiresLinearHistory:        n.RequiresLinearHistory,
				AllowsForcePushes:            n.AllowsForcePushes,
				AllowsDeletions:              n.AllowsDeletions,
				DismissesStaleReviews:        n.DismissesStaleReviews,
				RequiresCodeOwnerReviews:     n.RequiresCodeOwnerReviews,
			})
		}
		if !q.Repository.BranchProtectionRules.PageInfo.HasNextPage {
			break
		}
		variables["cursor"] = githubv4.NewString(q.Repository.BranchProtectionRules.PageInfo.EndCursor)
	}
	return rules, nil
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
		RepositoryID:                 repoID,
		Pattern:                      githubv4.String(pattern),
		RequiresApprovingReviews:     githubv4.NewBoolean(githubv4.Boolean(input.RequiresApprovingReviews)),
		RequiredApprovingReviewCount: githubv4.NewInt(githubv4.Int(input.RequiredApprovingReviewCount)),
		RequiresStatusChecks:         githubv4.NewBoolean(githubv4.Boolean(input.RequiresStatusChecks)),
		RequiresStrictStatusChecks:   githubv4.NewBoolean(githubv4.Boolean(input.RequiresStrictStatusChecks)),
		IsAdminEnforced:              githubv4.NewBoolean(githubv4.Boolean(input.IsAdminEnforced)),
		RequiresLinearHistory:        githubv4.NewBoolean(githubv4.Boolean(input.RequiresLinearHistory)),
		AllowsForcePushes:            githubv4.NewBoolean(githubv4.Boolean(input.AllowsForcePushes)),
		AllowsDeletions:              githubv4.NewBoolean(githubv4.Boolean(input.AllowsDeletions)),
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
		BranchProtectionRuleID:       ruleID,
		RequiresApprovingReviews:     githubv4.NewBoolean(githubv4.Boolean(input.RequiresApprovingReviews)),
		RequiredApprovingReviewCount: githubv4.NewInt(githubv4.Int(input.RequiredApprovingReviewCount)),
		RequiresStatusChecks:         githubv4.NewBoolean(githubv4.Boolean(input.RequiresStatusChecks)),
		RequiresStrictStatusChecks:   githubv4.NewBoolean(githubv4.Boolean(input.RequiresStrictStatusChecks)),
		IsAdminEnforced:              githubv4.NewBoolean(githubv4.Boolean(input.IsAdminEnforced)),
		RequiresLinearHistory:        githubv4.NewBoolean(githubv4.Boolean(input.RequiresLinearHistory)),
		AllowsForcePushes:            githubv4.NewBoolean(githubv4.Boolean(input.AllowsForcePushes)),
		AllowsDeletions:              githubv4.NewBoolean(githubv4.Boolean(input.AllowsDeletions)),
	}

	if err := c.GraphQL.Mutate(ctx, &m, gqlInput, nil); err != nil {
		return fmt.Errorf("updating branch protection rule %s: %w", ruleID, err)
	}
	return nil
}

// GetFileContent fetches the text content of a file from a repository via GraphQL.
// expression should be in the format "branch:path/to/file" (e.g., "main:CODEOWNERS").
func (c *Client) GetFileContent(ctx context.Context, repoName, expression string) (string, error) {
	var q struct {
		Repository struct {
			Object struct {
				Blob struct {
					Text string
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $expression)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":      githubv4.String(c.org),
		"name":       githubv4.String(repoName),
		"expression": githubv4.String(expression),
	}

	if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
		return "", fmt.Errorf("querying file content for %s/%s: %w", repoName, expression, err)
	}
	return q.Repository.Object.Blob.Text, nil
}

// FindMatchingProtectionRule finds the branch protection rule that applies to the given branch.
// Returns nil if no rule matches. Uses glob matching (e.g., "release/*" matches "release/1.0").
func FindMatchingProtectionRule(rules []BranchProtectionRule, branch string) *BranchProtectionRule {
	for i, r := range rules {
		matched, err := path.Match(r.Pattern, branch)
		if err == nil && matched {
			return &rules[i]
		}
	}
	return nil
}
