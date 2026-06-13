// Package telegram contains rolltonchatbot's Telegram update handlers.
package telegram

import (
	"fmt"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

type StartHandler struct{}

func NewStartHandler() *StartHandler { return &StartHandler{} }

// ReplyTextFor returns the welcome message. Exposed for tests; production
// path uses Handle below.
func (h *StartHandler) ReplyTextFor(u *domain.User) string {
	if u == nil || u.FirstName == "" {
		return "Welcome to rolltonchatbot. This is a scaffold reply — no behaviour yet."
	}
	return fmt.Sprintf("Welcome, %s — rolltonchatbot is ready.", u.FirstName)
}

func (h *StartHandler) Handle(c *tgbot.Context) error {
	u, _ := middleware.UserFromContext(c.Ctx())
	return c.Reply(h.ReplyTextFor(u))
}
