package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

// Character is a per-persona configuration used by the launcher.
// The base_prompt + a context.prompt form the LLM system prompt at runtime.
type Character struct {
	ID          uuid.UUID
	Slug        string
	Name        string
	Blurb       string
	AvatarURL   string // empty if not set
	BasePrompt  string
	BotUsername string // in C-mode always "rolltonchatbot"
	IsActive    bool
	Position    int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewCharacterFromPostgresRow builds a Character from a sqlc-generated row.
func NewCharacterFromPostgresRow(row postgres.Character) Character {
	return Character{
		ID:          uuid.UUID(row.ID.Bytes),
		Slug:        row.Slug,
		Name:        row.Name,
		Blurb:       row.Blurb,
		AvatarURL:   postgres.TextOrEmpty(row.AvatarUrl),
		BasePrompt:  row.BasePrompt,
		BotUsername: row.BotUsername,
		IsActive:    row.IsActive,
		Position:    row.Position,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}
