package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cashx/internal/platform"
	"cashx/internal/worker"
)

// worker: outbox delivery and daily stats aggregation.
func main() {
	cfg, err := platform.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	log := platform.NewLogger(cfg.Env)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := platform.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("worker starting")
	worker.New(db, cfg, log).Run(ctx)
	log.Info("worker stopped")
}
