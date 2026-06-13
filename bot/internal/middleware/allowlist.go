package middleware

import (
	"log/slog"

	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

// AllowOnlyUserIDs is a tgbot.Middleware that drops any update whose sender's
// Telegram ID is not in `allowed`. Drops include: (a) updates with no sender
// (channel posts, edits), (b) sender not in the allowlist, (c) empty allowlist.
// Rejected updates are logged at INFO and produce no reply.
func AllowOnlyUserIDs(allowed []int64, log *slog.Logger) tgbot.Middleware {
	set := make(map[int64]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
		return func(c *tgbot.Context) error {
			tg := c.Update.SentFrom()
			if tg == nil {
				log.Info("allowlist_drop_no_sender", "update_id", c.Update.UpdateID)
				return nil
			}
			if _, ok := set[tg.ID]; !ok {
				log.Info("allowlist_rejected",
					"telegram_id", tg.ID,
					"username", tg.UserName,
					"update_id", c.Update.UpdateID)
				return nil
			}
			return next(c)
		}
	}
}
