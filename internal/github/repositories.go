package github

import (
	"context"
	"fmt"

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

// ListRepositories fetches all repositories accessible to the GitHub App installation via GraphQL.
func (c *Client) ListRepositories(ctx context.Context) ([]Repository, error) {
	var q struct {
		Viewer struct {
			Repositories struct {
				Nodes []struct {
					ID               githubv4.ID
					Name             string
					Owner            struct{ Login string }
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
			} `graphql:"repositories(first: 100, after: $cursor, affiliations: [OWNER, ORGANIZATION_MEMBER, COLLABORATOR])"`
		}
	}

	variables := map[string]interface{}{
		"cursor": (*githubv4.String)(nil),
	}

	var repos []Repository
	for {
		if err := c.GraphQL.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("querying repositories: %w", err)
		}
		for _, n := range q.Viewer.Repositories.Nodes {
			repos = append(repos, Repository{
				ID:            n.ID.(string),
				Name:          n.Name,
				Owner:         n.Owner.Login,
				DefaultBranch: n.DefaultBranchRef.Name,
				IsArchived:    n.IsArchived,
				IsFork:        n.IsFork,
			})
		}
		if !q.Viewer.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["cursor"] = githubv4.NewString(q.Viewer.Repositories.PageInfo.EndCursor)
	}
	
	return repos, nil
}
