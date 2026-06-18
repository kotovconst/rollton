// Package ports declares service interfaces consumed by handlers + middleware.
// Concrete implementations live in internal/core/services (cross-bot) and
// internal/bots/<name>/services (bot-specific).
package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// UserService coordinates user registration across all bots.
// Implementations talk to storage directly; no separate repository layer.
type UserService interface {
	EnsureRegistered(ctx context.Context, user domain.User) (domain.User, error)
}

// CharacterService returns the catalog of characters the launcher offers.
// Today: a flat ListActive. Later: GetBySlug, ListByTag, etc.
type CharacterService interface {
	ListActive(ctx context.Context) ([]domain.Character, error)
}

// OpenRouterClient is the subset of *openrouter.Client that ChatFlowService
// uses. Declared here so tests can substitute a fake.
type OpenRouterClient interface {
	Complete(ctx context.Context, req openrouter.ChatRequest) (openrouter.ChatResponse, error)
}

// ReplyFunc is supplied by the Telegram handler and called by the service
// with the text to send back to the user. It returns the Telegram message id
// of the first sent chunk (for the assistant-msg insert) or an error if the
// send failed.
type ReplyFunc func(text string) (tgMessageID int64, err error)

// ChatFlowService runs one user turn end-to-end: persist user msg, build
// prompt, call OpenRouter, persist + send assistant reply.
type ChatFlowService interface {
	Handle(
		ctx context.Context,
		user domain.User,
		characterID uuid.UUID,
		text string,
		tgUserMessageID int64,
		reply ReplyFunc,
	) error
}
