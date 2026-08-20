package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cashx/internal/platform"
	"cashx/internal/tracking"
)

// redirect service: partner link clicks -> tracking_click + signed clickToken
// -> destination project.
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

	rdb, err := platform.NewRedis(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	handler := tracking.NewRedirectHandler(tracking.RedirectDeps{Cfg: cfg, Log: log, Pool: db, Redis: rdb})

	port := cfg.Port + 1
	if cfg.Env == "production" {
		port = 8081
	}
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Info("redirect listening", "addr", srv.Addr, "base", cfg.RedirectBase)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
