package github

import (
	"context"
	"fmt"

	"github.com/shurcooL/githubv4"
)

// Actor represents a GitHub actor (user, team, or app) resolved from a branch protection rule.
type Actor struct {
	Type string // "User", "Team", or "App"
	Name string // login, slug, or app slug
}

// actorNode is the GraphQL query shape for actor allowance nodes.
type actorNode struct {
	Actor struct {
		Team struct {
			Slug string
		} `graphql:"... on Team"`
		User struct {
			Login string
		} `graphql:"... on User"`
		App struct {
			Slug string
		} `graphql:"... on App"`
	}
}

func parseActors(nodes []actorNode) []Actor {
	var actors []Actor
	for _, n := range nodes {
		switch {
		case n.Actor.User.Login != "":
			actors = append(actors, Actor{Type: "User", Name: n.Actor.User.Login})
		case n.Actor.Team.Slug != "":
			actors = append(actors, Actor{Type: "Team", Name: n.Actor.Team.Slug})
		case n.Actor.App.Slug != "":
			actors = append(actors, Actor{Type: "App", Name: n.Actor.App.Slug})
		}
	}
	return actors
}

// ResolveUserID resolves a GitHub username to its node ID.
func (c *Client) ResolveUserID(ctx context.Context, login string) (string, error) {
	var q struct {
		User struct {
			ID githubv4.ID
		} `graphql:"user(login: $login)"`
	}
	variables := map[string]interface{}{
		"login": githubv4.String(login),
	}
	if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
		return "", fmt.Errorf("resolving user %q: %w", login, err)
	}
	return q.User.ID.(string), nil
}

// ResolveTeamID resolves an organization team slug to its node ID.
func (c *Client) ResolveTeamID(ctx context.Context, slug string) (string, error) {
	var q struct {
		Organization struct {
			Team struct {
				ID githubv4.ID
			} `graphql:"team(slug: $slug)"`
		} `graphql:"organization(login: $org)"`
	}
	variables := map[string]interface{}{
		"org":  githubv4.String(c.org),
		"slug": githubv4.String(slug),
	}
	if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
		return "", fmt.Errorf("resolving team %q: %w", slug, err)
	}
	return q.Organization.Team.ID.(string), nil
}

// ResolveActorIDs resolves lists of user logins and team slugs to their node IDs.
func (c *Client) ResolveActorIDs(ctx context.Context, users, teams []string) ([]string, error) {
	var ids []string
	for _, login := range users {
		id, err := c.ResolveUserID(ctx, login)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	for _, slug := range teams {
		id, err := c.ResolveTeamID(ctx, slug)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
