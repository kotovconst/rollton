// Package characterbots wires the per-character Telegram bots that run the
// chat-flow service. One binary, N bots (one goroutine per character),
// shared services + DB pool + OpenRouter client.
package characterbots

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	tgh "github.com/kotovconst/rollton/bot/internal/bots/characterbots/handlers/telegram"
	"github.com/kotovconst/rollton/bot/internal/config"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

type Deps struct {
	Cfg             config.Config
	Log             *slog.Logger
	Pool            *pgxpool.Pool
	UserSvc         ports.UserService
	ChatFlowHandler ports.ChatFlowHandlerFunc
}

// Run loads active characters from DB, starts one Telegram long-poll goroutine
// per character, and blocks until ctx is cancelled or any character goroutine
// returns an error. Missing BOT_TOKEN_* env var for an active character is
// fail-fast.
func Run(ctx context.Context, deps Deps) error {
	q := postgres.New(deps.Pool)
	chars, err := q.ListActiveCharacters(ctx)
	if err != nil {
		return fmt.Errorf("characterbots: load characters: %w", err)
	}
	if len(chars) == 0 {
		return fmt.Errorf("characterbots: no active characters in DB")
	}

	g, gctx := errgroup.WithContext(ctx)
	for _, ch := range chars {
		ch := ch
		envName := tokenEnvNameForSlug(ch.Slug)
		token := os.Getenv(envName)
		if token == "" {
			return fmt.Errorf("characterbots: missing env %s for character %q", envName, ch.Slug)
		}
		bot, err := tgbot.New(token)
		if err != nil {
			return fmt.Errorf("characterbots: connect %s: %w", ch.Slug, err)
		}
		bot.Use(middleware.Telegram(deps.Log))
		bot.Use(middleware.EnsureUserRegistered(deps.UserSvc, deps.Log))

		th := tgh.NewTextHandler(uuidFromPg(ch.ID), deps.ChatFlowHandler)
		bot.Router().HandleDefault(th.Handle)

		deps.Log.Info("character.started",
			"slug", ch.Slug, "username", ch.BotUsername)

		g.Go(func() error {
			err := bot.Run(gctx)
			if err != nil && err != context.Canceled {
				return fmt.Errorf("characterbots: %s run: %w", ch.Slug, err)
			}
			return nil
		})
	}
	return g.Wait()
}

// tokenEnvNameForSlug applies the documented rule: `-` → `_`, then upper-case.
//
//	"snoop-dogg"      → "BOT_TOKEN_SNOOP_DOGG"
//	"sherlock-holmes" → "BOT_TOKEN_SHERLOCK_HOLMES"
func tokenEnvNameForSlug(slug string) string {
	return "BOT_TOKEN_" + strings.ToUpper(strings.ReplaceAll(slug, "-", "_"))
}

func uuidFromPg(p pgtype.UUID) uuid.UUID { return uuid.UUID(p.Bytes) }
