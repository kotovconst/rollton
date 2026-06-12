package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	bota "github.com/kotovconst/rollton/bot/internal/bots/admin"
	"github.com/kotovconst/rollton/bot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "err", err)
		os.Exit(1)
	}

	log := newLogger(cfg.Log.Format, cfg.Log.Level).With("bot", "admin")

	app, err := bota.NewApp(bota.Deps{Cfg: cfg, Log: log})
	if err != nil {
		log.Error("app_init_failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
