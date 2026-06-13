package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	bota "github.com/kotovconst/rollton/bot/internal/bots/rolltonchatbot"
	"github.com/kotovconst/rollton/bot/internal/config"
	"github.com/kotovconst/rollton/bot/internal/core/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "err", err)
		os.Exit(1)
	}

	log := newLogger(cfg.Log.Format, cfg.Log.Level).With("bot", "rolltonchatbot")

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

	userSvc := services.NewUserService(pool)

	app, err := bota.NewApp(bota.Deps{Cfg: cfg, Log: log, UserSvc: userSvc})
	if err != nil {
		log.Error("app_init_failed", "err", err)
		os.Exit(1)
	}

	if err := app.Run(ctx); err != nil && err != context.Canceled {
		log.Error("app_run_failed", "err", err)
		os.Exit(1)
	}
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
