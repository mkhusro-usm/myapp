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
	configPath := flag.String("config", "config.yaml", "path to config file")
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
			continue
		}
		switch name {
		case "branch-protection":
			settings, err := rule.ParseSettings[rule.BranchProtectionSettings](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing branch_protection settings: %w", err)
			}
			a.registry.Register(rule.NewBranchProtection(a.client, settings))
		default:
			log.Printf("warning: unknown rule %q, skipping", name)
		}
	}
	return nil
}

// run fetches repos and evaluates/applies enabled rules in parallel.
func (a *App) run(ctx context.Context) error {
	repos, err := a.fetchRepos(ctx)
	if err != nil {
		return fmt.Errorf("fetching repos: %w", err)
	}

	log.Printf("targeting %d repository(ies) in %s (mode: %s)", len(repos), a.client.Org(), a.mode)
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
	if err := report.Write(a.outputPath); err != nil {
		log.Printf("error writing report: %v", err)
	}

	return nil
}

func (a *App) processRepo(ctx context.Context, repo *gh.Repository, rules map[string]rule.Rule) []*rule.Result {
	var results []*rule.Result
	for _, r := range rules {
		var result *rule.Result
		var err error

		if a.mode == rule.ModeApply {
			result, err = r.Apply(ctx, repo)
		} else {
			result, err = r.Evaluate(ctx, repo)
		}

		if err != nil {
			log.Printf("error processing %s on %s: %v", r.Name(), repo.FullName(), err)
			continue
		}
		results = append(results, result)
	}
	return results
}

func (a *App) fetchRepos(ctx context.Context) ([]gh.Repository, error) {
	if a.targetRepo != "" {
		repo, err := a.client.GetRepository(ctx, a.targetRepo)
		if err != nil {
			return nil, err
		}
		return []gh.Repository{*repo}, nil
	}
	
	return a.client.ListRepositories(ctx)
}

func loadPrivateKey(privateKeyPath string) (
	[]byte,
	error,
) {
	if key := os.Getenv("GH_APP_PRIVATE_KEY"); key != "" {
		return []byte(key), nil
	}

	if privateKeyPath == "" {
		return nil, fmt.Errorf("no private key: set GH_APP_PRIVATE_KEY env var or private_key_path in config")
	}

	return os.ReadFile(privateKeyPath)
}
