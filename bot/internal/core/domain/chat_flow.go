package domain

import (
	"time"

	"github.com/google/uuid"
)

// ChatFlowSnapshot bundles everything the chat-flow usecase needs about the
// current chat: identifying IDs, the character + context prompts that form
// the system prompt, and the model_config sampling parameters.
type ChatFlowSnapshot struct {
	ChatID              uuid.UUID
	ContextID           uuid.UUID
	CharacterID         uuid.UUID
	CharacterSlug       string
	CharacterBasePrompt string
	ContextPrompt       string
	ModelName           string
	Temperature         *float64
	TopP                *float64
	MaxTokens           *int32
}

// DefaultContextForChat is what ContextService returns when the usecase needs
// to bootstrap a brand-new chat (no historical chat row exists for the user
// and character).
type DefaultContextForChat struct {
	ContextID     uuid.UUID
	ContextSlug   string
	ContextPrompt string
	ModelConfigID uuid.UUID
	ModelSlug     string
	ModelName     string
	Temperature   *float64
	TopP          *float64
	MaxTokens     *int32
}

// UserMsgRecord is the minimal data returned when persisting (or looking up)
// a user message.
type UserMsgRecord struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

// RecentMessage is one entry of the LLM history window.
type RecentMessage struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

// ChatFlowErrClass labels the cause of a failed LLM round-trip for log
// aggregation and ops alerting.
type ChatFlowErrClass string

const (
	ChatFlowErrUnknown             ChatFlowErrClass = "unknown"
	ChatFlowErrInvalidAuth         ChatFlowErrClass = "invalid_auth"
	ChatFlowErrInsufficientCredits ChatFlowErrClass = "insufficient_credits"
	ChatFlowErrRateLimited         ChatFlowErrClass = "rate_limited"
	ChatFlowErrUpstream            ChatFlowErrClass = "upstream"
)
