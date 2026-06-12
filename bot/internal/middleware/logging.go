// Package middleware provides cross-cutting concerns for both HTTP and Telegram handlers.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

// Telegram returns a tgbot.Middleware that logs each handled update.
func Telegram(log *slog.Logger) tgbot.Middleware {
	return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
		return func(c *tgbot.Context) error {
			start := time.Now()
			err := next(c)
			attrs := []any{
				"update_id", c.Update.UpdateID,
				"user_id", c.UserID(),
				"chat_id", c.ChatID(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if err != nil {
				attrs = append(attrs, "err", err.Error())
				log.Error("telegram_update_failed", attrs...)
			} else {
				log.Info("telegram_update_handled", attrs...)
			}
			return err
		}
	}
}

// HTTP returns a net/http middleware that logs every request.
func HTTP(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r)
			log.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
