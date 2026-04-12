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

// App is the central struct holding all injected dependencies.
type App struct {
	Config    *config.Config
	Client    *gh.Client
	Transport *ghinstallation.Transport
	Registry  *rule.Registry
	Reporters []reporter.Reporter
}

// New constructs the App, wiring up all dependencies from config.
func New(cfg *config.Config) (*App, error) {
	key, err := loadPrivateKey(&cfg.GitHubApp)
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
		Config:    cfg,
		Client:    client,
		Transport: transport,
		Registry:  rule.NewRegistry(),
		Reporters: buildReporters(cfg),
	}

	if err := app.registerRules(); err != nil {
		return nil, fmt.Errorf("registering rules: %w", err)
	}

	return app, nil
}

func buildReporters(cfg *config.Config) []reporter.Reporter {
	var reporters []reporter.Reporter
	if cfg.Output.Console {
		reporters = append(reporters, &reporter.Console{})
	}
	if cfg.Output.JSON != "" {
		reporters = append(reporters, &reporter.JSON{OutputPath: cfg.Output.JSON})
	}
	return reporters
}

func (a *App) registerRules() error {
	for name, rc := range a.Config.Rules {
		if !rc.Enabled {
			continue
		}
		switch name {
		case "branch_protection":
			settings, err := rule.ParseSettings[rule.BranchProtectionSettings](rc.Settings)
			if err != nil {
				return fmt.Errorf("parsing branch_protection settings: %w", err)
			}
			a.Registry.Register(rule.NewBranchProtection(a.Client, settings))
		default:
			log.Printf("warning: unknown rule %q, skipping", name)
		}
	}
	return nil
}

// Run fetches repos and evaluates/applies enabled rules in parallel.
func (a *App) Run(ctx context.Context) error {
	repos, err := a.fetchRepos(ctx)
	if err != nil {
		return fmt.Errorf("fetching repos: %w", err)
	}

	log.Printf("targeting %d repository(ies) in %s (mode: %s)", len(repos), a.Client.Org(), a.Config.Mode)

	rules := a.Registry.All()
	if len(rules) == 0 {
		log.Println("no rules enabled, nothing to do")
		return nil
	}

	var (
		mu      sync.Mutex
		results []*rule.Result
		wg      sync.WaitGroup
		sem     = make(chan struct{}, a.Config.Concurrency)
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

			for _, r := range rules {
				var result *rule.Result
				var err error

				if a.Config.Mode == "apply" {
					result, err = r.Apply(ctx, repo)
				} else {
					result, err = r.Evaluate(ctx, repo)
				}

				if err != nil {
					log.Printf("error processing %s on %s: %v", r.Name(), repo.FullName(), err)
					continue
				}
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}
		}(&repos[i])
	}

	wg.Wait()

	report := reporter.BuildReport(a.Config.Org, a.Config.Mode, results)
	for _, r := range a.Reporters {
		if err := r.Report(ctx, report); err != nil {
			log.Printf("error writing report: %v", err)
		}
	}

	return nil
}

func (a *App) fetchRepos(ctx context.Context) ([]gh.Repository, error) {
	if a.Config.TargetRepo != "" {
		repo, err := a.Client.GetRepository(ctx, a.Config.TargetRepo)
		if err != nil {
			return nil, err
		}
		return []gh.Repository{*repo}, nil
	}
	return a.Client.ListRepositories(ctx)
}

func loadPrivateKey(cfg *config.GitHubAppConfig) ([]byte, error) {
	if key := os.Getenv("GITHUB_APP_PRIVATE_KEY"); key != "" {
		return []byte(key), nil
	}
	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("no private key: set GITHUB_APP_PRIVATE_KEY env var or private_key_path in config")
	}
	return os.ReadFile(cfg.PrivateKeyPath)
}
