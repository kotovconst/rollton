// Package domain holds bot-agnostic entities. No DB, no Telegram, no HTTP.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrUserNotFound is returned when no row matches a lookup by telegram_id.
// Services translate driver errors (e.g. pgx.ErrNoRows) to this.
var ErrUserNotFound = errors.New("user not found")

// User is the internal representation. Lifetime: stored, never recreated.
//
// Construct fresh instances from incoming Telegram data via NewUser.
// Loaded instances come from the storage layer with all fields populated.
type User struct {
	ID           uuid.UUID
	TelegramID   int64
	Username     string // empty if user has no @handle in Telegram
	FirstName    string
	LastName     string // empty if not set
	LanguageCode string // e.g. "en", "ru"; empty if not advertised
	IsPremium    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser builds a User from incoming Telegram update fields. The ID and
// timestamps are left zero — the storage layer populates them.
func NewUser(telegramID int64, username, firstName, lastName, languageCode string, isPremium bool) User {
	return User{
		TelegramID:   telegramID,
		Username:     username,
		FirstName:    firstName,
		LastName:     lastName,
		LanguageCode: languageCode,
		IsPremium:    isPremium,
	}
}

// TelegramFieldsMatch reports whether u and other carry the same
// Telegram-sourced fields. The service uses this to decide whether to skip
// an upsert when the incoming update brings nothing new.
func (u User) TelegramFieldsMatch(other User) bool {
	return u.Username == other.Username &&
		u.FirstName == other.FirstName &&
		u.LastName == other.LastName &&
		u.LanguageCode == other.LanguageCode &&
		u.IsPremium == other.IsPremium
}
