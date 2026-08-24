package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cashx/internal/api"
	"cashx/internal/auth"
	"cashx/internal/platform"
	"cashx/internal/repository"

	"github.com/thecodearcher/limen"
)

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
	log.Info("database connected")

	rdb, err := platform.NewRedis(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()
	log.Info("redis connected")

	// Integration key encryption must be configured outside dev.
	if cfg.Env != "development" && cfg.IntegrationKeyEncryptionKey == "" {
		log.Error("CASHX_INTEGRATION_KEY_ENCRYPTION_KEY is required outside development")
		os.Exit(1)
	}

	limenAuth, err := auth.New(cfg, db)
	if err != nil {
		log.Error("init limen auth", "err", err)
		os.Exit(1)
	}

	seedAdmin(ctx, log, cfg, limenAuth)

	srv := api.New(cfg, log, db, limenAuth)
	handler := srv.Router(rdb)

	httpSrv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}

// seedAdmin creates the initial superadmin when no users exist.
func seedAdmin(ctx context.Context, log *slog.Logger, cfg platform.Config, limenAuth *auth.Limen) {
	pool, err := platform.NewDB(ctx, cfg.AdminDatabaseURL)
	if err != nil {
		log.Error("seed admin: connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	repo := repository.New(pool)
	n, err := repo.CountUsers(ctx)
	if err != nil {
		log.Warn("seed admin: count users failed", "err", err)
		return
	}
	if n > 0 {
		return
	}
	// Seed through Limen so the password hash format matches signin.
	pwd := cfg.AdminPassword
	res, err := limenAuth.Password.SignUpWithCredentialAndPassword(ctx,
		&limen.User{Email: cfg.AdminEmail, Password: &pwd},
		map[string]any{"name": "Администратор", "role": "staff", "is_active": true},
	)
	if err != nil {
		log.Error("seed admin failed", "err", err)
		os.Exit(1)
	}
	if err := repo.CreateStaffRoleAssignment(ctx, repository.CreateStaffRoleAssignmentParams{
		UserID: fmt.Sprint(res.User.ID), Role: "superadmin", ProjectID: repository.UUIDPtr(nil),
	}); err != nil {
		log.Error("seed admin: role assignment failed", "err", err)
		os.Exit(1)
	}
	refCode, err := auth.GenerateReferralCode(ctx, repo)
	if err != nil {
		log.Error("seed admin: referral code failed", "err", err)
		os.Exit(1)
	}
	profile, err := repo.CreatePartnerProfile(ctx, repository.CreatePartnerProfileParams{
		UserID:       fmt.Sprint(res.User.ID),
		ReferralCode: refCode,
		ReferredBy:   repository.UUIDPtr(nil),
	})
	if err != nil {
		log.Error("seed admin: partner profile failed", "err", err)
		os.Exit(1)
	}
	approved := true
	if _, err := repo.UpdatePartnerProfile(ctx, repository.UpdatePartnerProfileParams{
		ID: profile.ID, IsApproved: repository.BoolPtr(&approved),
	}); err != nil {
		log.Error("seed admin: partner profile approval failed", "err", err)
		os.Exit(1)
	}
	if _, err := repo.CreateWallet(ctx, profile.ID); err != nil {
		log.Error("seed admin: wallet failed", "err", err)
		os.Exit(1)
	}
	log.Info("seeded superadmin", "email", cfg.AdminEmail)
}
