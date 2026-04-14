package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/mkhusro-usm/myapp/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("error: %v", err)
	}
}
