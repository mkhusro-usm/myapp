package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mkhusro-usm/myapp/app"
	"github.com/mkhusro-usm/myapp/config"
	"github.com/mkhusro-usm/myapp/rule"
)

const (
	defaultConfigPath   = "config.yaml"
	defaultOverridesDir = "overrides"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	configPath := flag.String("config", defaultConfigPath, "path to config file")
	repo := flag.String("repo", "", "target a single repository (e.g. my-repo)")
	mode := flag.String("mode", "evaluate", "run mode: evaluate or apply")
	output := flag.String("output", "", "path to write JSON report (e.g. reports/compliance.json)")
	flag.Parse()

	// Load config.
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load per-repo overrides.
	overrides, err := config.LoadOverrides(defaultOverridesDir)
	if err != nil {
		return fmt.Errorf("loading overrides: %w", err)
	}
	if len(overrides) > 0 {
		log.Printf("loaded overrides for %d repository(ies)", len(overrides))
	}

	// Parse run mode.
	m, err := rule.ParseMode(*mode)
	if err != nil {
		return err
	}

	// Build options.
	opts := []app.Option{
		app.WithMode(m),
	}
	if *repo != "" {
		opts = append(opts, app.WithTargetRepo(*repo))
	}
	if *output != "" {
		opts = append(opts, app.WithOutput(*output))
	}

	// Construct the app.
	application, err := app.New(cfg, overrides, opts...)
	if err != nil {
		return fmt.Errorf("initializing app: %w", err)
	}

	// Run.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Handle cancellation and timeout.
	go func() {
		<-ctx.Done()
		log.Println("context cancelled or timed out, shutting down...")
		os.Exit(1)
	}()

	// Run with timeout.
	runCtx, runCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer runCancel()

	return application.Run(runCtx)
}
