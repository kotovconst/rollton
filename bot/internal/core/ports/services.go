// Package ports declares service interfaces consumed by handlers + middleware.
// Concrete implementations live in internal/core/services (cross-bot) and
// internal/bots/<name>/services (bot-specific).
package ports

import (
	"context"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
)

// UserService coordinates user registration across all bots.
// Implementations talk to storage directly; no separate repository layer.
type UserService interface {
	EnsureRegistered(ctx context.Context, user domain.User) (domain.User, error)
}
