package tgbot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps tgbotapi.BotAPI with a Router and middleware chain.
type Bot struct {
	api    *tgbotapi.BotAPI
	router *Router
	mws    []Middleware
}

// New connects to Telegram and returns a configured Bot.
func New(token string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("tgbot: empty token")
	}
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("tgbot: connect: %w", err)
	}
	return &Bot{api: api, router: NewRouter()}, nil
}

// Use appends middlewares (outer-most first).
func (b *Bot) Use(mws ...Middleware) { b.mws = append(b.mws, mws...) }

// Router exposes the internal router for handler registration.
func (b *Bot) Router() *Router { return b.router }

// API exposes the underlying client.
func (b *Bot) API() *tgbotapi.BotAPI { return b.api }

// Run starts the long-poll loop. Returns when ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	handler := Chain(b.router.Dispatch, b.mws...)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case upd, ok := <-updates:
			if !ok {
				return nil
			}
			c := newContext(ctx, b.api, upd)
			_ = handler(c) // logging middleware is responsible for surfacing errors
		}
	}
}
