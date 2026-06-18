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

// PtrFloat64 returns nil for invalid pgtype.Float8, else a pointer to Float64.
func PtrFloat64(p pgtype.Float8) *float64 {
	if !p.Valid {
		return nil
	}
	v := p.Float64
	return &v
}

// PtrInt32 returns nil for invalid pgtype.Int4, else a pointer to Int32.
func PtrInt32(p pgtype.Int4) *int32 {
	if !p.Valid {
		return nil
	}
	v := p.Int32
	return &v
}
