// Package services contains cross-bot use cases. Implementations follow the
// viza_assigment pattern: each service holds a sqlc *Queries directly,
// translates DB errors inline, and is constructed from a DBTX-compatible
// connection (in production: *pgxpool.Pool; in tests: pgxmock).
package services

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

type userService struct {
	queries *postgres.Queries
}

// NewUserService wires the user service against any postgres.DBTX-compatible
// connection. Use *pgxpool.Pool in production, pgxmock in tests.
func NewUserService(db postgres.DBTX) ports.UserService {
	return &userService{queries: postgres.New(db)}
}

// EnsureRegistered loads the row by telegram_id, returns it as-is if every
// Telegram-sourced field matches, otherwise upserts the incoming user.
// pgx.ErrNoRows is treated as "user is new" — straight to upsert.
func (s *userService) EnsureRegistered(ctx context.Context, user domain.User) (domain.User, error) {
	stored, err := s.queries.GetUserByTelegramID(ctx, user.TelegramID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, errors.Join(errors.New("getting user"), err)
	}
	if err == nil {
		existing := postgres.ToDomainUser(stored)
		if existing.TelegramFieldsMatch(user) {
			return existing, nil
		}
	}

	row, err := s.queries.UpsertUserFromTelegram(ctx, postgres.UpsertUserFromTelegramParams{
		TelegramID:   user.TelegramID,
		Username:     postgres.StringToText(user.Username),
		FirstName:    user.FirstName,
		LastName:     postgres.StringToText(user.LastName),
		LanguageCode: postgres.StringToText(user.LanguageCode),
		IsPremium:    user.IsPremium,
	})
	if err != nil {
		return domain.User{}, errors.Join(errors.New("upserting user"), err)
	}
	return postgres.ToDomainUser(row), nil
}
