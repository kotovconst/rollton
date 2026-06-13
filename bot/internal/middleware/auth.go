package middleware

import (
	"context"
	"log/slog"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

// UserService is the subset of services.UserService that the middleware needs.
// Declared here so the middleware doesn't import the services package directly
// (loosens the dependency graph; eases testing with fakes).
type UserService interface {
	EnsureRegistered(ctx context.Context, input domain.TelegramUserInput) (domain.User, error)
}

type userCtxKey struct{}

// WithUser returns a new context carrying u.
func WithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, u)
}

// UserFromContext returns the registered user, or (nil, false) if none was set.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(userCtxKey{}).(*domain.User)
	return u, ok
}

// EnsureUserRegistered is a tgbot.Middleware that auto-registers the sender on
// every update. If SentFrom() is nil (channel posts, edits without user) it
// passes through. If the service errors, it logs and continues — handlers can
// fall back when no user is in context.
func EnsureUserRegistered(svc UserService, log *slog.Logger) tgbot.Middleware {
	return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
		return func(c *tgbot.Context) error {
			tg := c.Update.SentFrom()
			if tg == nil {
				return next(c)
			}
			input := domain.TelegramUserInput{
				TelegramID:   tg.ID,
				Username:     tg.UserName,
				FirstName:    tg.FirstName,
				LastName:     tg.LastName,
				LanguageCode: tg.LanguageCode,
				// IsPremium: tgbotapi/v5 v5.5.1 doesn't expose this field;
				// stub leaves it false until the library is bumped or replaced.
				IsPremium: false,
			}
			u, err := svc.EnsureRegistered(c.Ctx(), input)
			if err != nil {
				log.Error("user_registration_failed",
					"telegram_id", tg.ID, "err", err)
				return next(c)
			}
			c.SetCtx(WithUser(c.Ctx(), &u))
			return next(c)
		}
	}
}
