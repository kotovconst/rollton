package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

type contextService struct {
	queries *postgres.Queries
}

func NewContextService(db postgres.DBTX) ports.ContextService {
	return &contextService{queries: postgres.New(db)}
}

func (s *contextService) GetDefaultWithModelForCharacter(ctx context.Context, characterID uuid.UUID) (domain.DefaultContextForChat, error) {
	row, err := s.queries.GetDefaultContextWithModelForCharacter(ctx, pgtype.UUID{Bytes: characterID, Valid: true})
	if err != nil {
		return domain.DefaultContextForChat{}, err
	}
	return domain.DefaultContextForChat{
		ContextID:     uuid.UUID(row.ContextID.Bytes),
		ContextSlug:   row.ContextSlug,
		ContextPrompt: row.ContextPrompt,
		ModelConfigID: uuid.UUID(row.ModelConfigID.Bytes),
		ModelSlug:     row.ModelSlug,
		ModelName:     row.ModelName,
		Temperature:   postgres.PtrFloat64(row.Temperature),
		TopP:          postgres.PtrFloat64(row.TopP),
		MaxTokens:     postgres.PtrInt32(row.MaxTokens),
	}, nil
}
