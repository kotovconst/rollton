package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"

	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

// TextHandler dispatches inbound text messages to ChatFlowService.
// One instance per character (holds the character id).
type TextHandler struct {
	characterID uuid.UUID
	svc         ports.ChatFlowService
}

func NewTextHandler(characterID uuid.UUID, svc ports.ChatFlowService) *TextHandler {
	return &TextHandler{characterID: characterID, svc: svc}
}

func (h *TextHandler) Handle(c *tgbot.Context) error {
	msg := c.Update.Message
	if msg == nil || msg.Text == "" {
		return nil
	}
	u, ok := middleware.UserFromContext(c.Ctx())
	if !ok || u == nil {
		return nil
	}
	reply := func(text string) (int64, error) {
		return sendReply(c, msg.Chat.ID, text)
	}
	return tgbot.WithTyping(c, func() error {
		return h.svc.Handle(c.Ctx(), *u, h.characterID, msg.Text, int64(msg.MessageID), reply)
	})
}

// sendReply chunks long text and sends each chunk via Telegram. Returns the
// MessageID of the first sent chunk; subsequent chunk IDs are not persisted.
func sendReply(c *tgbot.Context, chatID int64, text string) (int64, error) {
	chunks := ChunkText(text, TelegramMessageMaxBytes)
	if len(chunks) == 0 {
		return 0, nil
	}
	api := c.API()
	if api == nil {
		return 0, nil // test context with no api — no-op send
	}
	var firstID int64
	for i, chunk := range chunks {
		sent, err := api.Send(tgbotapi.NewMessage(chatID, chunk))
		if err != nil {
			return 0, err
		}
		if i == 0 {
			firstID = int64(sent.MessageID)
		}
	}
	return firstID, nil
}
