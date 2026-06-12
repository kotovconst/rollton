// Package telegram contains bot_b's Telegram update handlers.
package telegram

import (
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

type StartHandler struct{}

func NewStartHandler() *StartHandler { return &StartHandler{} }

// ReplyText is the exact message /start sends. Extracted so it's testable
// without a real Telegram API.
func (h *StartHandler) ReplyText() string {
	return "Welcome to bot_b. This is a scaffold reply — no behaviour yet."
}

// Handle wires the handler into the tgbot router.
func (h *StartHandler) Handle(c *tgbot.Context) error {
	return c.Reply(h.ReplyText())
}
