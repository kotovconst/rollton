package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kotovconst/rollton/bot/internal/bots/characterbots"
	"github.com/kotovconst/rollton/bot/internal/config"
	"github.com/kotovconst/rollton/bot/internal/core/services"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "err", err)
		os.Exit(1)
	}

	log := newLogger(cfg.Log.Format, cfg.Log.Level).With("bot", "characterbots")

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
		log.Warn("db_ping_failed", "err", err)
	}
	cancelPing()

	userSvc := services.NewUserService(pool)
	orClient := openrouter.New(
		cfg.OpenRouter.APIKey,
		openrouter.WithApp(cfg.OpenRouter.AppURL, cfg.OpenRouter.AppName),
	)
	chatFlowSvc := services.NewChatFlowService(pool, orClient, cfg.LLMHistoryWindow, log)

	if err := characterbots.Run(ctx, characterbots.Deps{
		Cfg:         cfg,
		Log:         log,
		Pool:        pool,
		UserSvc:     userSvc,
		ChatFlowSvc: chatFlowSvc,
	}); err != nil && err != context.Canceled {
		log.Error("characterbots_run_failed", "err", err)
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
