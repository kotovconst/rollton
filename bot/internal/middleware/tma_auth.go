package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	httpx "github.com/kotovconst/rollton/bot/pkg/http"
)

// TmaAuth gates /api/v1/* with Telegram Mini App initData HMAC validation.
// Reads "Authorization: tma <initDataRaw>", verifies via init-data-golang,
// calls svc.EnsureRegistered to materialize the user, and puts *domain.User
// into the request context for downstream handlers.
//
// Any HMAC/parse/freshness failure produces 401 UNAUTHORIZED. The body does
// not distinguish (security best practice — don't help triangulation).
func TmaAuth(botToken string, svc UserService, log *slog.Logger, maxAge time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := parseTmaHeader(r.Header.Get("Authorization"))
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "missing tma credentials")
				return
			}
			if err := initdata.Validate(raw, botToken, maxAge); err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "invalid init data")
				return
			}
			user, ok := parseUserFromInitData(raw)
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "malformed init data")
				return
			}
			u, err := svc.EnsureRegistered(r.Context(), user)
			if err != nil {
				log.Error("http_user_register_failed", "telegram_id", user.TelegramID, "err", err)
				httpx.WriteError(w, http.StatusInternalServerError, httpx.ErrCodeInternalError, "registration failed")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), &u)))
		})
	}
}

func parseTmaHeader(h string) (string, bool) {
	const prefix = "tma "
	if len(h) <= len(prefix) || !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return h[len(prefix):], true
}

// parseUserFromInitData pulls the user JSON out of the validated initData
// query string and builds a domain.User. The lib's InitData.User is a value
// type — a zero ID indicates no user was provided.
func parseUserFromInitData(raw string) (domain.User, bool) {
	parsed, err := initdata.Parse(raw)
	if err != nil {
		return domain.User{}, false
	}
	if parsed.User.ID == 0 {
		return domain.User{}, false
	}
	u := parsed.User
	return domain.NewUser(
		u.ID,
		u.Username,
		u.FirstName,
		u.LastName,
		u.LanguageCode,
		u.IsPremium,
	), true
}
