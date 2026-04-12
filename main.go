package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/mkhusro-usm/myapp/app"
	"github.com/mkhusro-usm/myapp/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	repo := flag.String("repo", "", "target a single repository (e.g. my-repo)")
	mode := flag.String("mode", "", "override mode from config (evaluate or apply)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	if *mode != "" {
		cfg.Mode = *mode
	}
	if *repo != "" {
		cfg.TargetRepo = *repo
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("initializing app: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("running app: %v", err)
	}
}
