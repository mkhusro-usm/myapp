package rule

import (
	"context"

	gogithub "github.com/google/go-github/v84/github"
	gh "github.com/mkhusro-usm/myapp/internal/github"
)

// stubRepoSettingsClient implements RepoSettingsClient for tests.
type stubRepoSettingsClient struct {
	settings  *gh.RepoSettings
	getErr    error
	updateErr error
}

func (s *stubRepoSettingsClient) GetRepoSettings(_ context.Context, _ string) (*gh.RepoSettings, error) {
	return s.settings, s.getErr
}

func (s *stubRepoSettingsClient) UpdateRepoSettings(_ context.Context, _ string, _ *gh.RepoSettings) error {
	return s.updateErr
}

// stubCodeownersClient implements CodeownersClient for tests.
type stubCodeownersClient struct {
	content string
	getErr  error
	prURL   string
	prErr   error
}

func (s *stubCodeownersClient) GetFileContent(_ context.Context, _, _, _ string) (string, error) {
	return s.content, s.getErr
}

func (s *stubCodeownersClient) CreateFileChangePR(_ context.Context, _, _, _, _, _ string, _ []gh.FileChange) (string, error) {
	return s.prURL, s.prErr
}

// stubRulesetsClient implements RulesetsClient for tests.
type stubRulesetsClient struct {
	ruleset   *gogithub.RepositoryRuleset
	getErr    error
	created   *gogithub.RepositoryRuleset
	createErr error
	updated   *gogithub.RepositoryRuleset
	updateErr error
}

func (s *stubRulesetsClient) GetRepoRulesetByName(_ context.Context, _, _ string) (*gogithub.RepositoryRuleset, error) {
	return s.ruleset, s.getErr
}

func (s *stubRulesetsClient) CreateRepoRuleset(_ context.Context, _ string, _ gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	return s.created, s.createErr
}

func (s *stubRulesetsClient) UpdateRepoRuleset(_ context.Context, _ string, _ int64, _ gogithub.RepositoryRuleset) (*gogithub.RepositoryRuleset, error) {
	return s.updated, s.updateErr
}
