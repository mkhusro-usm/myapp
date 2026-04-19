// Package app provides the main application logic for governance enforcement.
//
// The App loads configuration, authenticates as a GitHub App, and executes governance
// rules in two phases: org-scoped rules first, then repo-scoped rules.
// Results are collected and written as a JSON report.
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"

	"github.com/mkhusro-usm/myapp/config"
	gh "github.com/mkhusro-usm/myapp/internal/github"
	"github.com/mkhusro-usm/myapp/reporter"
	"github.com/mkhusro-usm/myapp/rule"
)

const envPrivateKey = "GH_APP_PRIVATE_KEY"

// App is the central application struct holding all dependencies.
// It coordinates rule execution across repositories and produces reports.
type App struct {
	config     *config.Config
	client     *gh.Client
	transport  *ghinstallation.Transport
	registry   *rule.Registry
	mode       rule.Mode
	targetRepo string
	outputPath string
}

// Option is a functional option for configuring an App instance.
type Option func(*App)

// WithTargetRepo scopes the run to a single repository.
func WithTargetRepo(repo string) Option {
	return func(a *App) {
		a.targetRepo = repo
	}
}

// WithMode sets the run mode (evaluate or apply).
func WithMode(mode rule.Mode) Option {
	return func(a *App) {
		a.mode = mode
	}
}

// WithOutput sets the output file path for the JSON report.
// If not set, the report is written to stdout.
func WithOutput(path string) Option {
	return func(a *App) {
		a.outputPath = path
	}
}

// New constructs a fully initialized App from the given config, overrides, and options.
// It sets up the GitHub App client, registers all enabled rules, and returns an App
// ready to execute via Run. Per-repo overrides are passed through to individual rules
// that support them — the App itself does not interpret overrides.
func New(cfg *config.Config, overrides map[string]config.RepoOverride, opts ...Option) (*App, error) {
	key, err := loadPrivateKey(cfg.GitHubApp.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading private key: %w", err)
	}

	transport, err := ghinstallation.New(
		http.DefaultTransport,
		cfg.GitHubApp.AppID,
		cfg.GitHubApp.InstallationID,
		key,
	)
	if err != nil {
		return nil, fmt.Errorf("creating github app transport: %w", err)
	}

	httpClient := &http.Client{Transport: transport}
	client := gh.NewClient(httpClient, cfg.Org)

	a := &App{
		config:    cfg,
		client:    client,
		transport: transport,
		registry:  rule.NewRegistry(),
		mode:      rule.ModeEvaluate,
	}

	for _, opt := range opts {
		opt(a)
	}

	if err := a.registerRules(overrides); err != nil {
		return nil, fmt.Errorf("registering rules: %w", err)
	}

	return a, nil
}

// Run executes governance rules in two phases: org-scoped rules first, then repo-scoped rules.
func (a *App) Run(ctx context.Context) error {
	repoRules := a.registry.RepoRules()
	orgRules := a.registry.OrgRules()

	if len(repoRules) == 0 && len(orgRules) == 0 {
		log.Println("no rules enabled, nothing to do")
		return nil
	}

	var orgResults []*rule.Result
	var repoResults []*rule.Result

	// Phase 1: org-scoped rules (run once, not per-repo).
	if len(orgRules) > 0 {
		log.Printf("running %d org-scoped rule(s) (mode: %s)", len(orgRules), a.mode)
		orgResults = a.processOrgRules(ctx, orgRules)
	}

	// Phase 2: repo-scoped rules (run per-repo, concurrently).
	if len(repoRules) > 0 {
		log.Printf("fetching repositories for org %s", a.client.Org())
		repos, err := a.fetchRepos(ctx)
		if err != nil {
			return fmt.Errorf("fetching repos: %w", err)
		}

		log.Printf("found %d repository(ies) to process with %d repo-scoped rule(s) (mode: %s)", len(repos), len(repoRules), a.mode)

		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			sem = make(chan struct{}, a.config.Concurrency)
		)

		for i := range repos {
			if repos[i].IsArchived {
				continue
			}
			wg.Add(1)
			go func(repo *gh.Repository) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				results := a.processRepo(ctx, repo, repoRules)
				mu.Lock()
				repoResults = append(repoResults, results...)
				mu.Unlock()
			}(&repos[i])
		}
		wg.Wait()
	}

	report := reporter.BuildReport(a.config.Org, a.mode, orgResults, repoResults)

	log.Printf("run complete: %d repositories, %d evaluations, %d compliant, %d non-compliant, %d applied",
		report.Summary.Repositories, report.Summary.TotalEvaluations, report.Summary.CompliantResults, report.Summary.NonCompliantResults, report.Summary.AppliedResults)
	if len(report.Summary.PullRequests) > 0 {
		log.Printf("pull requests created: %d", len(report.Summary.PullRequests))
	}

	if a.outputPath != "" {
		log.Printf("writing report to %s", a.outputPath)
	}

	if err := report.Write(a.outputPath); err != nil {
		log.Printf("error writing report: %v", err)
	}

	return nil
}

// loadPrivateKey retrieves the GitHub App private key.
// It first checks the environment variable, then falls back to reading the file.
func loadPrivateKey(privateKeyPath string) ([]byte, error) {
	if key := os.Getenv(envPrivateKey); key != "" {
		return []byte(key), nil
	}

	if privateKeyPath == "" {
		return nil, fmt.Errorf("no private key: set %s env var or private-key-path in config", envPrivateKey)
	}

	return os.ReadFile(privateKeyPath)
}

// registerRules iterates over the config rules and registers enabled ones.
// Per-repo overrides are passed to each rule's constructor for rule-specific handling.
func (a *App) registerRules(overrides map[string]config.RepoOverride) error {
	for name, rc := range a.config.Rules {
		if !rc.Enabled {
			log.Printf("rule %q is disabled, skipping", name)
			continue
		}
		if rc.Scope == "" {
			return fmt.Errorf("rule %q is missing required 'scope' field (repo or org)", name)
		}
		log.Printf("registering rule: %s (scope: %s)", name, rc.Scope)
		switch name {
		case "repo-rulesets":
			settings, err := rule.ParseSettings[rule.RulesetsSettings](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing rulesets settings: %w", err)
			}
			a.registry.RegisterRepoRule(rule.NewRepoRulesets(a.client, settings))
		case "codeowners":
			settings, err := rule.ParseSettings[rule.CodeownersSettings](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing codeowners settings: %w", err)
			}
			a.registry.RegisterRepoRule(rule.NewCodeowners(a.client, settings, overrides))
		case "repo-settings":
			settings, err := rule.ParseSettings[rule.RepoSettingsConfig](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing repo-settings settings: %w", err)
			}
			a.registry.RegisterRepoRule(rule.NewRepoSettings(a.client, settings))
		default:
			log.Printf("warning: unknown rule %q, skipping", name)
		}
	}

	return nil
}

// processOrgRules runs all org-scoped rules and collects their results.
func (a *App) processOrgRules(ctx context.Context, rules map[string]rule.OrgRule) []*rule.Result {
	var results []*rule.Result
	for _, r := range rules {
		log.Printf("[org] running %s (%s)", r.Name(), a.mode)
		var result *rule.Result
		var err error

		if a.mode == rule.ModeApply {
			result, err = r.Apply(ctx)
		} else {
			result, err = r.Evaluate(ctx)
		}

		if err != nil {
			log.Printf("[org] error in %s: %v", r.Name(), err)
			continue
		}

		a.logResult("org", r.Name(), result)
		results = append(results, result)
	}
	return results
}

// fetchRepos returns the list of repositories to process.
// If targetRepo is set, it returns only that repository; otherwise it lists all accessible repos.
func (a *App) fetchRepos(ctx context.Context) ([]gh.Repository, error) {
	if a.targetRepo != "" {
		log.Printf("targeting single repository: %s", a.targetRepo)
		repo, err := a.client.GetRepository(ctx, a.targetRepo)
		if err != nil {
			return nil, err
		}
		return []gh.Repository{*repo}, nil
	}
	log.Printf("listing all repositories accessible to the GitHub App")

	return a.client.ListRepositories(ctx)
}

// processRepo runs all repo-scoped rules against a single repository and collects their results.
func (a *App) processRepo(ctx context.Context, repo *gh.Repository, rules map[string]rule.RepoRule) []*rule.Result {
	log.Printf("[%s] processing repository (%d rules)", repo.FullName(), len(rules))
	var results []*rule.Result

	for _, r := range rules {
		log.Printf("[%s] running %s (%s)", repo.FullName(), r.Name(), a.mode)
		var result *rule.Result
		var err error

		if a.mode == rule.ModeApply {
			result, err = r.Apply(ctx, repo)
		} else {
			result, err = r.Evaluate(ctx, repo)
		}

		if err != nil {
			log.Printf("[%s] error in %s: %v", repo.FullName(), r.Name(), err)
			continue
		}

		a.logResult(repo.FullName(), r.Name(), result)
		results = append(results, result)
	}
	return results
}

// logResult logs the outcome of a rule evaluation.
func (a *App) logResult(scope, ruleName string, result *rule.Result) {
	if result.Compliant {
		log.Printf("[%s] %s: compliant", scope, ruleName)
	} else {
		log.Printf("[%s] %s: non-compliant (%d violations)", scope, ruleName, result.ViolationCount)
	}
	if result.Applied {
		log.Printf("[%s] %s: applied successfully", scope, ruleName)
	}
}
