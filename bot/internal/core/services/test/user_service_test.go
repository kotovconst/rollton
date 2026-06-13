package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/services"
)

// userRowColumns mirrors the order of fields in the sqlc-generated User struct.
func userRowColumns() []string {
	return []string{"id", "telegram_id", "username", "first_name", "last_name", "language_code", "is_premium", "created_at", "updated_at"}
}

func toRow(u domain.User) []any {
	uuidBytes := [16]byte(u.ID)
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	return []any{
		pgtype.UUID{Bytes: uuidBytes, Valid: true},
		u.TelegramID,
		textOrNullable(u.Username),
		u.FirstName,
		textOrNullable(u.LastName),
		textOrNullable(u.LanguageCode),
		u.IsPremium,
		pgtype.Timestamptz{Time: u.CreatedAt, Valid: true},
		pgtype.Timestamptz{Time: u.UpdatedAt, Valid: true},
	}
}

func textOrNullable(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func newMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	m, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(m.Close)
	return m
}

func TestEnsureRegistered_FoundMatching_SkipsUpsert(t *testing.T) {
	mock := newMock(t)
	input := domain.NewUser(42, "alice", "Alice", "Smith", "en", false)
	stored := input
	stored.ID = uuid.New()

	mock.ExpectQuery(`SELECT (.+) FROM users WHERE telegram_id`).
		WithArgs(int64(42)).
		WillReturnRows(pgxmock.NewRows(userRowColumns()).AddRow(toRow(stored)...))

	svc := services.NewUserService(mock)
	u, err := svc.EnsureRegistered(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, stored.ID, u.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureRegistered_FieldDrift_TriggersUpsert(t *testing.T) {
	mock := newMock(t)
	input := domain.NewUser(42, "alice", "Alice", "Smith", "en", false)
	stored := input
	stored.ID = uuid.New()
	stored.Username = "old_handle" // drifted

	mock.ExpectQuery(`SELECT (.+) FROM users WHERE telegram_id`).
		WithArgs(int64(42)).
		WillReturnRows(pgxmock.NewRows(userRowColumns()).AddRow(toRow(stored)...))

	upserted := input
	upserted.ID = stored.ID
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(input.TelegramID, textOrNullable(input.Username), input.FirstName,
			textOrNullable(input.LastName), textOrNullable(input.LanguageCode), input.IsPremium).
		WillReturnRows(pgxmock.NewRows(userRowColumns()).AddRow(toRow(upserted)...))

	svc := services.NewUserService(mock)
	u, err := svc.EnsureRegistered(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, "alice", u.Username)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureRegistered_NotFound_CallsUpsert(t *testing.T) {
	mock := newMock(t)
	input := domain.NewUser(42, "alice", "Alice", "Smith", "en", false)

	mock.ExpectQuery(`SELECT (.+) FROM users WHERE telegram_id`).
		WithArgs(int64(42)).
		WillReturnError(pgx.ErrNoRows)

	created := input
	created.ID = uuid.New()
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(input.TelegramID, textOrNullable(input.Username), input.FirstName,
			textOrNullable(input.LastName), textOrNullable(input.LanguageCode), input.IsPremium).
		WillReturnRows(pgxmock.NewRows(userRowColumns()).AddRow(toRow(created)...))

	svc := services.NewUserService(mock)
	u, err := svc.EnsureRegistered(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, int64(42), u.TelegramID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureRegistered_GetErrorOtherThanNotFound_Bubbles(t *testing.T) {
	mock := newMock(t)
	input := domain.NewUser(42, "alice", "Alice", "Smith", "en", false)

	dbErr := errors.New("connection refused")
	mock.ExpectQuery(`SELECT (.+) FROM users WHERE telegram_id`).
		WithArgs(int64(42)).
		WillReturnError(dbErr)

	svc := services.NewUserService(mock)
	_, err := svc.EnsureRegistered(context.Background(), input)

	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
