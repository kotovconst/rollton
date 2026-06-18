// Package ports declares service interfaces consumed by handlers, middleware,
// and usecases.
//
// Layering convention:
//   - **services** wrap data operations (sqlc queries) and return domain types.
//     They contain no logging and do not orchestrate multiple operations.
//   - **usecases** orchestrate services for a single business action. They own
//     logging, error classification, and (where appropriate) DB transactions.
//   - **handlers** call usecases.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// UserService coordinates user registration across all bots.
type UserService interface {
	EnsureRegistered(ctx context.Context, user domain.User) (domain.User, error)
}

// CharacterService returns characters.
type CharacterService interface {
	ListActive(ctx context.Context) ([]domain.Character, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Character, error)
}

// ChatService wraps the `chats` table.
type ChatService interface {
	// GetMostRecentForUserCharacter returns the most-recent chat for the given
	// user across any context belonging to characterID, hydrated as a snapshot.
	// Returns the underlying driver's no-rows sentinel when none exists.
	GetMostRecentForUserCharacter(ctx context.Context, userID, characterID uuid.UUID) (domain.ChatFlowSnapshot, error)
	// Create inserts a new chat row and returns its id.
	Create(ctx context.Context, userID, contextID uuid.UUID) (uuid.UUID, error)
	// TouchUpdatedAt bumps chats.updated_at so the most-recent-chat lookup works.
	TouchUpdatedAt(ctx context.Context, chatID uuid.UUID) error
}

// ContextService wraps the `contexts` table.
type ContextService interface {
	// GetDefaultWithModelForCharacter returns the lowest-position active context
	// for characterID, joined with its model_config.
	GetDefaultWithModelForCharacter(ctx context.Context, characterID uuid.UUID) (domain.DefaultContextForChat, error)
}

// InsertAssistantArgs is the input to MessageService.InsertAssistant.
type InsertAssistantArgs struct {
	ChatID            uuid.UUID
	Content           string
	TelegramMessageID int64
	Model             string
	TokensIn          int32
	TokensOut         int32
}

// MessageService wraps the `tg_messages` table.
type MessageService interface {
	// InsertUserIdempotent attempts to insert a user message; returns the
	// driver's no-rows sentinel when ON CONFLICT DO NOTHING swallows the row.
	InsertUserIdempotent(ctx context.Context, chatID uuid.UUID, content string, tgMessageID int64) (domain.UserMsgRecord, error)
	// GetUserByTGID fetches an existing user-role message by (chat, telegram id).
	GetUserByTGID(ctx context.Context, chatID uuid.UUID, tgMessageID int64) (domain.UserMsgRecord, error)
	// AssistantReplyExistsAfter is true iff any assistant row exists in chat
	// with created_at strictly greater than the given timestamp.
	AssistantReplyExistsAfter(ctx context.Context, chatID uuid.UUID, after time.Time) (bool, error)
	// ListRecent returns the last `limit` user/assistant messages in DESC order
	// by created_at (newest-first). Callers reverse for chronological ordering.
	ListRecent(ctx context.Context, chatID uuid.UUID, limit int32) ([]domain.RecentMessage, error)
	// InsertAssistant persists a successful LLM reply with usage metadata.
	InsertAssistant(ctx context.Context, args InsertAssistantArgs) error
}

// OpenRouterClient is the subset of *openrouter.Client that the chat-flow
// usecase uses. Declared here so tests can substitute a fake.
type OpenRouterClient interface {
	Complete(ctx context.Context, req openrouter.ChatRequest) (openrouter.ChatResponse, error)
}

// ReplyFunc is supplied by the Telegram handler and called by the chat-flow
// usecase with text to send back to the user. Returns the first chunk's
// telegram message id (used for the assistant-msg insert) or an error if the
// send failed.
type ReplyFunc func(text string) (tgMessageID int64, err error)

// ChatFlowHandlerFunc is the handler-facing entry point for one user turn.
// The chat-flow usecase produces a value of this type bound to its
// dependencies; the Telegram TextHandler calls it.
type ChatFlowHandlerFunc func(
	ctx context.Context,
	user domain.User,
	characterID uuid.UUID,
	text string,
	tgUserMessageID int64,
	reply ReplyFunc,
) error
