// Package ports declares interfaces consumed by services. Concrete
// implementations live in pkg/sqlc/postgres (adapters) and tests (fakes).
package ports

import (
	"context"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
)

// UserRepository is the persistence boundary for User.
// Implementations must translate driver-specific "row not found" errors to
// domain.ErrUserNotFound. All other errors bubble up as-is.
type UserRepository interface {
	GetByTelegramID(ctx context.Context, tgID int64) (domain.User, error)
	UpsertFromTelegram(ctx context.Context, input domain.TelegramUserInput) (domain.User, error)
}
