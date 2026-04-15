package app

import (
	"context"
	"flag"
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

const (
	defaultConfigPath = "config.yaml"
	envPrivateKey     = "GH_APP_PRIVATE_KEY"
)

// Option is a functional option for configuring the App.
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

// App is the central struct holding all injected dependencies.
type App struct {
	config     *config.Config
	client     *gh.Client
	transport  *ghinstallation.Transport
	registry   *rule.Registry
	mode       rule.Mode
	targetRepo string
	outputPath string
}

// Run is the top-level entry point. It parses flags, loads config, and executes the app.
func Run(ctx context.Context) error {
	configPath := flag.String("config", defaultConfigPath, "path to config file")
	repo := flag.String("repo", "", "target a single repository (e.g. my-repo)")
	mode := flag.String("mode", "evaluate", "run mode: evaluate or apply")
	output := flag.String("output", "", "path to write JSON report (e.g. reports/compliance.json)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	m, err := rule.ParseMode(*mode)
	if err != nil {
		return err
	}

	opts := []Option{
		WithMode(m),
	}

	if *repo != "" {
		opts = append(opts, WithTargetRepo(*repo))
	}

	if *output != "" {
		opts = append(opts, WithOutput(*output))
	}

	application, err := newApp(cfg, opts...)
	if err != nil {
		return fmt.Errorf("initializing app: %w", err)
	}

	return application.run(ctx)
}

// newApp constructs the App, wiring up all dependencies from config.
func newApp(cfg *config.Config, opts ...Option) (*App, error) {
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

	log.Printf("authenticated as GitHub App (app-id: %d, installation-id: %d)", cfg.GitHubApp.AppID, cfg.GitHubApp.InstallationID)

	app := &App{
		config:    cfg,
		client:    client,
		transport: transport,
		registry:  rule.NewRegistry(),
		mode:      rule.ModeEvaluate,
	}

	for _, opt := range opts {
		opt(app)
	}

	if err := app.registerRules(); err != nil {
		return nil, fmt.Errorf("registering rules: %w", err)
	}

	return app, nil
}

func (a *App) registerRules() error {
	for name, rc := range a.config.Rules {
		if !rc.Enabled {
			log.Printf("rule %q is disabled, skipping", name)
			continue
		}
		log.Printf("registering rule: %s", name)
		switch name {
		case "branch-protection":
			settings, err := rule.ParseSettings[rule.BranchProtectionSettings](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing branch-protection settings: %w", err)
			}
			a.registry.Register(rule.NewBranchProtection(a.client, settings))
		case "codeowners":
			settings, err := rule.ParseSettings[rule.CodeownersSettings](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing codeowners settings: %w", err)
			}
			a.registry.Register(rule.NewCodeowners(a.client, settings))
		case "repo-settings":
			settings, err := rule.ParseSettings[rule.RepoSettingsConfig](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing repo-settings settings: %w", err)
			}
			a.registry.Register(rule.NewRepoSettings(a.client, settings))
		default:
			log.Printf("warning: unknown rule %q, skipping", name)
		}
	}
	return nil
}

// run fetches repos and evaluates/applies enabled rules in parallel.
func (a *App) run(ctx context.Context) error {
	log.Printf("fetching repositories for org %s", a.client.Org())
	repos, err := a.fetchRepos(ctx)
	if err != nil {
		return fmt.Errorf("fetching repos: %w", err)
	}

	log.Printf("found %d repository(ies) to process (mode: %s)", len(repos), a.mode)
	rules := a.registry.All()
	if len(rules) == 0 {
		log.Println("no rules enabled, nothing to do")
		return nil
	}

	var (
		mu      sync.Mutex
		results []*rule.Result
		wg      sync.WaitGroup
		sem     = make(chan struct{}, a.config.Concurrency)
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

			repoResults := a.processRepo(ctx, repo, rules)
			mu.Lock()
			results = append(results, repoResults...)
			mu.Unlock()
		}(&repos[i])
	}
	wg.Wait()

	report := reporter.BuildReport(a.config.Org, a.mode, results)

	log.Printf("run complete: %d total, %d compliant, %d non-compliant, %d applied",
		report.Summary.Total, report.Summary.Compliant, report.Summary.NonCompliant, report.Summary.Applied)
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

func (a *App) processRepo(ctx context.Context, repo *gh.Repository, rules map[string]rule.Rule) []*rule.Result {
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

		if result.Compliant {
			log.Printf("[%s] %s: compliant", repo.FullName(), r.Name())
		} else {
			log.Printf("[%s] %s: non-compliant (%d violations)", repo.FullName(), r.Name(), result.ViolationCount)
		}
		if result.Applied {
			log.Printf("[%s] %s: applied successfully", repo.FullName(), r.Name())
		}

		results = append(results, result)
	}

	return results
}

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

func loadPrivateKey(privateKeyPath string) (
	[]byte,
	error,
) {
	if key := os.Getenv(envPrivateKey); key != "" {
		return []byte(key), nil
	}

	if privateKeyPath == "" {
		return nil, fmt.Errorf("no private key: set %s env var or private-key-path in config", envPrivateKey)
	}

	return os.ReadFile(privateKeyPath)
}
