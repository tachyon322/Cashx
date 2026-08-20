package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pressly/goose/v3"

	"cashx/internal/platform"

	// Register the pgx database/sql driver used by goose.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// migrate applies goose migrations from ./migrations using the admin database
// role (CASHX_ADMIN_DATABASE_URL / CASHX_DATABASE_URL).
func main() {
	cfg, err := platform.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	log := platform.NewLogger(cfg.Env)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", cfg.AdminDatabaseURL)
	if err != nil {
		log.Error("open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	goose.SetBaseFS(os.DirFS("migrations"))
	if err := goose.SetDialect("postgres"); err != nil {
		log.Error("set dialect", "err", err)
		os.Exit(1)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		log.Error("migrate up", "err", err)
		os.Exit(1)
	}
	log.Info("migrations applied")
}
