package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

type chatService struct {
	queries *postgres.Queries
}

func NewChatService(db postgres.DBTX) ports.ChatService {
	return &chatService{queries: postgres.New(db)}
}

func (s *chatService) GetMostRecentForUserCharacter(ctx context.Context, userID, characterID uuid.UUID) (domain.ChatFlowSnapshot, error) {
	row, err := s.queries.GetMostRecentChatJoinedForUserCharacter(ctx, postgres.GetMostRecentChatJoinedForUserCharacterParams{
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		ID:     pgtype.UUID{Bytes: characterID, Valid: true},
	})
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	return domain.ChatFlowSnapshot{
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
	}, nil
}

func (s *chatService) Create(ctx context.Context, userID, contextID uuid.UUID) (uuid.UUID, error) {
	row, err := s.queries.InsertChat(ctx, postgres.InsertChatParams{
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		ContextID: pgtype.UUID{Bytes: contextID, Valid: true},
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return uuid.UUID(row.ID.Bytes), nil
}

func (s *chatService) TouchUpdatedAt(ctx context.Context, chatID uuid.UUID) error {
	return s.queries.TouchChatUpdatedAt(ctx, pgtype.UUID{Bytes: chatID, Valid: true})
}
