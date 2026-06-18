package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

// ChatFlowSnapshot bundles everything ChatFlowService.Handle needs about the
// current chat: identifying IDs, the character + context prompts that form the
// system prompt, and the model_config sampling parameters.
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

func NewChatFlowSnapshotFromJoinedRow(row postgres.GetMostRecentChatJoinedForUserCharacterRow) ChatFlowSnapshot {
	return ChatFlowSnapshot{
		ChatID:              uuid.UUID(row.ChatID.Bytes),
		ContextID:           uuid.UUID(row.ContextID.Bytes),
		CharacterID:         uuid.UUID(row.CharacterID.Bytes),
		CharacterSlug:       row.CharacterSlug,
		CharacterBasePrompt: row.CharacterBasePrompt,
		ContextPrompt:       row.ContextPrompt,
		ModelName:           row.ModelName,
		Temperature:         postgres.PtrFloat64(row.Temperature),
		TopP:                postgres.PtrFloat64(row.TopP),
		MaxTokens:           postgres.PtrInt32(row.MaxTokens),
	}
}

// NewChatFlowSnapshotFromContext builds a snapshot for a freshly-created chat
// (no historical chat row to load).
func NewChatFlowSnapshotFromContext(
	chatID uuid.UUID,
	characterID uuid.UUID,
	characterSlug string,
	characterBasePrompt string,
	defaultCtx postgres.GetDefaultContextWithModelForCharacterRow,
) ChatFlowSnapshot {
	return ChatFlowSnapshot{
		ChatID:              chatID,
		ContextID:           uuid.UUID(defaultCtx.ContextID.Bytes),
		CharacterID:         characterID,
		CharacterSlug:       characterSlug,
		CharacterBasePrompt: characterBasePrompt,
		ContextPrompt:       defaultCtx.ContextPrompt,
		ModelName:           defaultCtx.ModelName,
		Temperature:         postgres.PtrFloat64(defaultCtx.Temperature),
		TopP:                postgres.PtrFloat64(defaultCtx.TopP),
		MaxTokens:           postgres.PtrInt32(defaultCtx.MaxTokens),
	}
}

// RecentMessage is one entry of the LLM history window.
type RecentMessage struct {
	Role      string
	Content   string
	CreatedAt time.Time
}
