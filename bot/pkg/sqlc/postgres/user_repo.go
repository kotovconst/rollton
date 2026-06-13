package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
)

// UserRepoPg implements ports.UserRepository using sqlc-generated queries.
type UserRepoPg struct {
	q *Queries
}

func NewUserRepoPg(q *Queries) *UserRepoPg {
	return &UserRepoPg{q: q}
}

func (r *UserRepoPg) GetByTelegramID(ctx context.Context, tgID int64) (domain.User, error) {
	row, err := r.q.GetUserByTelegramID(ctx, tgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("user_repo: get: %w", err)
	}
	return toDomainUser(row), nil
}

func (r *UserRepoPg) UpsertFromTelegram(ctx context.Context, input domain.TelegramUserInput) (domain.User, error) {
	params := UpsertUserFromTelegramParams{
		TelegramID:   input.TelegramID,
		Username:     stringToText(input.Username),
		FirstName:    input.FirstName,
		LastName:     stringToText(input.LastName),
		LanguageCode: stringToText(input.LanguageCode),
		IsPremium:    input.IsPremium,
	}
	row, err := r.q.UpsertUserFromTelegram(ctx, params)
	if err != nil {
		return domain.User{}, fmt.Errorf("user_repo: upsert: %w", err)
	}
	return toDomainUser(row), nil
}

func toDomainUser(row User) domain.User {
	return domain.User{
		ID:           uuid.UUID(row.ID.Bytes),
		TelegramID:   row.TelegramID,
		Username:     textOrEmpty(row.Username),
		FirstName:    row.FirstName,
		LastName:     textOrEmpty(row.LastName),
		LanguageCode: textOrEmpty(row.LanguageCode),
		IsPremium:    row.IsPremium,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func stringToText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
