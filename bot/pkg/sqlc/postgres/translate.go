package postgres

import (
	"github.com/jackc/pgx/v5/pgtype"
)

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
