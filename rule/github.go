package rule

import (
	"context"

	gogithub "github.com/google/go-github/v84/github"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// RepoSettingsClient is the subset of the GitHub client needed by the repo-settings rule.
type RepoSettingsClient interface {
	GetRepoSettings(ctx context.Context, repoName string) (*gh.RepoSettings, error)
	UpdateRepoSettings(ctx context.Context, repoName string, s *gh.RepoSettings) error
}

// CodeownersClient is the subset of the GitHub client needed by the codeowners rule.
type CodeownersClient interface {
	GetFileContent(ctx context.Context, repoName, branch, path string) (string, error)
	CreateFileChangePR(ctx context.Context, repoName, baseBranch, branchName, prTitle, prBody string, changes []gh.FileChange) (string, error)
}

// RulesetsClient is the subset of the GitHub client needed by the repo-rulesets rule.
type RulesetsClient interface {
	GetRepoRulesetByName(ctx context.Context, repoName, name string) (*gogithub.RepositoryRuleset, error)
	CreateRepoRuleset(ctx context.Context, repoName string, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error)
	UpdateRepoRuleset(ctx context.Context, repoName string, rulesetID int64, rs gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error)
}
