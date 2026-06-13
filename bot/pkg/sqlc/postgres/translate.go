package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
)

// ToDomainUser converts a sqlc-generated User row into a domain.User.
// Hand-written companion to the generated models — kept here so any service
// that touches User rows can use the same translator.
func ToDomainUser(row User) domain.User {
	return domain.User{
		ID:           uuid.UUID(row.ID.Bytes),
		TelegramID:   row.TelegramID,
		Username:     TextOrEmpty(row.Username),
		FirstName:    row.FirstName,
		LastName:     TextOrEmpty(row.LastName),
		LanguageCode: TextOrEmpty(row.LanguageCode),
		IsPremium:    row.IsPremium,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

// StringToText turns an empty string into a NULL Text, non-empty into a valid Text.
func StringToText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// TextOrEmpty extracts the string from pgtype.Text, returning "" if NULL.
func TextOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
