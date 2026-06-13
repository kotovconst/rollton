package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	bota "github.com/kotovconst/rollton/bot/internal/bots/admin"
	"github.com/kotovconst/rollton/bot/internal/config"
	"github.com/kotovconst/rollton/bot/internal/core/services"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "err", err)
		os.Exit(1)
	}

	log := newLogger(cfg.Log.Format, cfg.Log.Level).With("bot", "admin")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		log.Error("db_pool_init_failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		log.Warn("db_ping_failed", "err", err) // continue: long-poll still works
	}
	cancelPing()

	queries := postgres.New(pool)
	userRepo := postgres.NewUserRepoPg(queries)
	userSvc := services.NewUserService(userRepo)

	allowedIDs := parseAllowedUserIDs(os.Getenv("ADMIN_ALLOWED_USER_IDS"))
	log.Info("admin_allowlist_loaded", "count", len(allowedIDs))

	app, err := bota.NewApp(bota.Deps{
		Cfg:            cfg,
		Log:            log,
		UserSvc:        userSvc,
		AllowedUserIDs: allowedIDs,
	})
	if err != nil {
		log.Error("app_init_failed", "err", err)
		os.Exit(1)
	}

	if err := app.Run(ctx); err != nil && err != context.Canceled {
		log.Error("app_run_failed", "err", err)
		os.Exit(1)
	}
}

// parseAllowedUserIDs reads ADMIN_ALLOWED_USER_IDS (comma-separated int64s).
// Empty string → empty slice → middleware rejects everyone.
func parseAllowedUserIDs(raw string) []int64 {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, n)
	}
	return ids
}

func newLogger(format, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if format == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}
