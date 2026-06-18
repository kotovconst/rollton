package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

func TestNewChatFlowSnapshotFromJoinedRow(t *testing.T) {
	chatID := uuid.New()
	ctxID := uuid.New()
	charID := uuid.New()
	mcID := uuid.New()
	now := time.Now().UTC()

	row := postgres.GetMostRecentChatJoinedForUserCharacterRow{
		ChatID:              pgtype.UUID{Bytes: chatID, Valid: true},
		ChatStatus:          "active",
		ChatSummary:         pgtype.Text{},
		ChatUpdatedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		ContextID:           pgtype.UUID{Bytes: ctxID, Valid: true},
		ContextSlug:         "studio",
		ContextName:         "In the Studio",
		ContextPrompt:       "Setting: studio.",
		IsAgeRestricted:     false,
		CharacterID:         pgtype.UUID{Bytes: charID, Valid: true},
		CharacterSlug:       "snoop-dogg",
		CharacterName:       "Snoop Dogg",
		CharacterBasePrompt: "You are Snoop.",
		ModelConfigID:       pgtype.UUID{Bytes: mcID, Valid: true},
		ModelSlug:           "fast",
		ModelName:           "anthropic/claude-haiku-4.5",
		Temperature:         pgtype.Float8{Float64: 0.7, Valid: true},
		TopP:                pgtype.Float8{},
		MaxTokens:           pgtype.Int4{Int32: 256, Valid: true},
	}

	snap := domain.NewChatFlowSnapshotFromJoinedRow(row)

	require.Equal(t, chatID, snap.ChatID)
	require.Equal(t, ctxID, snap.ContextID)
	require.Equal(t, charID, snap.CharacterID)
	require.Equal(t, "snoop-dogg", snap.CharacterSlug)
	require.Equal(t, "You are Snoop.", snap.CharacterBasePrompt)
	require.Equal(t, "Setting: studio.", snap.ContextPrompt)
	require.Equal(t, "anthropic/claude-haiku-4.5", snap.ModelName)
	require.NotNil(t, snap.Temperature)
	require.InDelta(t, 0.7, *snap.Temperature, 1e-9)
	require.Nil(t, snap.TopP)
	require.NotNil(t, snap.MaxTokens)
	require.Equal(t, int32(256), *snap.MaxTokens)
}
