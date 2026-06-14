package services

import (
	"context"
	"errors"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

type characterService struct {
	queries *postgres.Queries
}

// NewCharacterService wires the character service against any postgres.DBTX.
func NewCharacterService(db postgres.DBTX) ports.CharacterService {
	return &characterService{queries: postgres.New(db)}
}

func (s *characterService) ListActive(ctx context.Context) ([]domain.Character, error) {
	rows, err := s.queries.ListActiveCharacters(ctx)
	if err != nil {
		return nil, errors.Join(errors.New("listing characters"), err)
	}
	out := make([]domain.Character, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.NewCharacterFromPostgresRow(row))
	}
	return out, nil
}
