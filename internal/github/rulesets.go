package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v84/github"
)

// GetRepoRulesetByName fetches a repository ruleset by name (excluding inherited org rulesets).
// Returns nil (without error) if no ruleset with the given name exists.
func (c *Client) GetRepoRulesetByName(ctx context.Context, repoName, name string) (*gogithub.RepositoryRuleset, error) {
	includeParents := false
	opts := &gogithub.RepositoryListRulesetsOptions{
		IncludesParents: &includeParents,
	}

	for {
		rulesets, resp, err := c.restClient.Repositories.GetAllRulesets(ctx, c.org, repoName, opts)
		if err != nil {
			return nil, fmt.Errorf("listing repo rulesets for %s: %w", repoName, err)
		}
		logRateLimit(resp)

		for _, rs := range rulesets {
			if rs.Name == name {
				full, resp, err := c.restClient.Repositories.GetRuleset(ctx, c.org, repoName, rs.GetID(), false)
				if err != nil {
					return nil, fmt.Errorf("getting repo ruleset %q for %s: %w", name, repoName, err)
				}
				logRateLimit(resp)
				return full, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return nil, nil
}

// CreateRepoRuleset creates a repository-level ruleset.
func (c *Client) CreateRepoRuleset(ctx context.Context, repoName string, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	created, resp, err := c.restClient.Repositories.CreateRuleset(ctx, c.org, repoName, rs)
	if err != nil {
		return nil, fmt.Errorf("creating repo ruleset %q for %s: %w", rs.Name, repoName, err)
	}
	logRateLimit(resp)
	return created, nil
}

// UpdateRepoRuleset updates an existing repository-level ruleset.
func (c *Client) UpdateRepoRuleset(ctx context.Context, repoName string, rulesetID int64, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	updated, resp, err := c.restClient.Repositories.UpdateRuleset(ctx, c.org, repoName, rulesetID, rs)
	if err != nil {
		return nil, fmt.Errorf("updating repo ruleset %d for %s: %w", rulesetID, repoName, err)
	}
	logRateLimit(resp)
	return updated, nil
}
