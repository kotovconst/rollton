// Package admin composes the admin bot from its handlers and shared deps.
package admin

import (
	"context"
	"fmt"
	"log/slog"
	httpstd "net/http"
	"time"

	httph "github.com/kotovconst/rollton/bot/internal/bots/admin/handlers/http"
	tgh "github.com/kotovconst/rollton/bot/internal/bots/admin/handlers/telegram"
	"github.com/kotovconst/rollton/bot/internal/config"
	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
	"golang.org/x/sync/errgroup"
)

type Deps struct {
	Cfg config.Config
	Log *slog.Logger
}

type App struct {
	deps Deps
	bot  *tgbot.Bot
	mux  *httpstd.ServeMux
}

func NewApp(deps Deps) (*App, error) {
	b, err := tgbot.New(deps.Cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("admin: %w", err)
	}
	b.Use(middleware.Telegram(deps.Log))

	start := tgh.NewStartHandler()
	b.Router().Handle("start", start.Handle)

	mux := httpstd.NewServeMux()
	mux.Handle("/healthz", httph.NewHealthzHandler())

	return &App{deps: deps, bot: b, mux: mux}, nil
}

// Run starts long-poll + HTTP. Returns when ctx is cancelled or either exits with error.
func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	srv := &httpstd.Server{
		Addr:              fmt.Sprintf(":%d", a.deps.Cfg.HTTP.Port),
		Handler:           middleware.HTTP(a.deps.Log)(a.mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	g.Go(func() error {
		a.deps.Log.Info("http_listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != httpstd.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		a.deps.Log.Info("telegram_polling")
		err := a.bot.Run(gctx)
		if err != nil && err != context.Canceled {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	return g.Wait()
}
