package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v84/github"
)

// GetOrgRulesetByName fetches an organization ruleset by name.
// Returns nil (without error) if no ruleset with the given name exists.
func (c *Client) GetOrgRulesetByName(ctx context.Context, name string) (*gogithub.RepositoryRuleset, error) {
	rulesets, _, err := c.REST.Organizations.GetAllRepositoryRulesets(ctx, c.org, nil)
	if err != nil {
		return nil, fmt.Errorf("listing org rulesets: %w", err)
	}
	for _, rs := range rulesets {
		if rs.Name == name {
			full, _, err := c.REST.Organizations.GetRepositoryRuleset(ctx, c.org, rs.GetID())
			if err != nil {
				return nil, fmt.Errorf("getting org ruleset %q: %w", name, err)
			}
			return full, nil
		}
	}
	return nil, nil
}

// CreateOrgRuleset creates an organization-level ruleset.
func (c *Client) CreateOrgRuleset(ctx context.Context, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	created, _, err := c.REST.Organizations.CreateRepositoryRuleset(ctx, c.org, rs)
	if err != nil {
		return nil, fmt.Errorf("creating org ruleset %q: %w", rs.Name, err)
	}
	return created, nil
}

// UpdateOrgRuleset updates an existing organization-level ruleset.
func (c *Client) UpdateOrgRuleset(ctx context.Context, rulesetID int64, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	updated, _, err := c.REST.Organizations.UpdateRepositoryRuleset(ctx, c.org, rulesetID, rs)
	if err != nil {
		return nil, fmt.Errorf("updating org ruleset %d: %w", rulesetID, err)
	}
	return updated, nil
}

// GetRepoRulesetByName fetches a repository ruleset by name (excluding inherited org rulesets).
// Returns nil (without error) if no ruleset with the given name exists.
func (c *Client) GetRepoRulesetByName(ctx context.Context, repoName, name string) (*gogithub.RepositoryRuleset, error) {
	includeParents := false
	rulesets, _, err := c.REST.Repositories.GetAllRulesets(ctx, c.org, repoName, &gogithub.RepositoryListRulesetsOptions{
		IncludesParents: &includeParents,
	})
	if err != nil {
		return nil, fmt.Errorf("listing repo rulesets for %s: %w", repoName, err)
	}
	for _, rs := range rulesets {
		if rs.Name == name {
			full, _, err := c.REST.Repositories.GetRuleset(ctx, c.org, repoName, rs.GetID(), false)
			if err != nil {
				return nil, fmt.Errorf("getting repo ruleset %q for %s: %w", name, repoName, err)
			}
			return full, nil
		}
	}
	return nil, nil
}

// CreateRepoRuleset creates a repository-level ruleset.
func (c *Client) CreateRepoRuleset(ctx context.Context, repoName string, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	created, _, err := c.REST.Repositories.CreateRuleset(ctx, c.org, repoName, rs)
	if err != nil {
		return nil, fmt.Errorf("creating repo ruleset %q for %s: %w", rs.Name, repoName, err)
	}
	return created, nil
}

// UpdateRepoRuleset updates an existing repository-level ruleset.
func (c *Client) UpdateRepoRuleset(ctx context.Context, repoName string, rulesetID int64, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	updated, _, err := c.REST.Repositories.UpdateRuleset(ctx, c.org, repoName, rulesetID, rs)
	if err != nil {
		return nil, fmt.Errorf("updating repo ruleset %d for %s: %w", rulesetID, repoName, err)
	}
	return updated, nil
}
