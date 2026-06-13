// Package domain holds bot-agnostic entities. No DB, no Telegram, no HTTP.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrUserNotFound is returned by UserRepository.GetByTelegramID when no row
// matches. Adapters translate driver errors (e.g. pgx.ErrNoRows) to this.
var ErrUserNotFound = errors.New("user not found")

// User is the internal representation. Lifetime: stored, never recreated.
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

// TelegramUserInput is the data we read off an incoming update.
// Built by the middleware from update.SentFrom().
type TelegramUserInput struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	IsPremium    bool
}
