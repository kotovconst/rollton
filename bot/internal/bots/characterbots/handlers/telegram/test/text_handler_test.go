package telegram_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	tgh "github.com/kotovconst/rollton/bot/internal/bots/characterbots/handlers/telegram"
	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

type chatFlowRecorder struct {
	mu          sync.Mutex
	calls       int32
	lastUser    domain.User
	lastCharID  uuid.UUID
	lastText    string
	lastTgMsgID int64
}

func (r *chatFlowRecorder) handler() ports.ChatFlowHandlerFunc {
	return func(_ context.Context, u domain.User, charID uuid.UUID, text string, tgMsgID int64, _ ports.ReplyFunc) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		atomic.AddInt32(&r.calls, 1)
		r.lastUser = u
		r.lastCharID = charID
		r.lastText = text
		r.lastTgMsgID = tgMsgID
		return nil
	}
}

func TestTextHandler_DispatchesText(t *testing.T) {
	charID := uuid.New()
	rec := &chatFlowRecorder{}
	h := tgh.NewTextHandler(charID, rec.handler())

	user := &domain.User{ID: uuid.New(), TelegramID: 99}
	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 5555,
			Text:      "hello",
			From:      &tgbotapi.User{ID: 99},
			Chat:      &tgbotapi.Chat{ID: 99},
		},
	}
	c := tgbot.NewTestContext(middleware.WithUser(context.Background(), user), upd)

	require.NoError(t, h.Handle(c))
	require.Equal(t, int32(1), atomic.LoadInt32(&rec.calls))
	require.Equal(t, user.ID, rec.lastUser.ID)
	require.Equal(t, charID, rec.lastCharID)
	require.Equal(t, "hello", rec.lastText)
	require.Equal(t, int64(5555), rec.lastTgMsgID)
}

func TestTextHandler_NoMessage_NoOp(t *testing.T) {
	rec := &chatFlowRecorder{}
	h := tgh.NewTextHandler(uuid.New(), rec.handler())

	upd := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{Data: "x"}}
	c := tgbot.NewTestContext(context.Background(), upd)

	require.NoError(t, h.Handle(c))
	require.Equal(t, int32(0), atomic.LoadInt32(&rec.calls))
}

func TestTextHandler_EmptyText_NoOp(t *testing.T) {
	rec := &chatFlowRecorder{}
	h := tgh.NewTextHandler(uuid.New(), rec.handler())

	user := &domain.User{ID: uuid.New(), TelegramID: 99}
	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 1,
			From:      &tgbotapi.User{ID: 99},
			Chat:      &tgbotapi.Chat{ID: 99},
			Sticker:   &tgbotapi.Sticker{},
		},
	}
	c := tgbot.NewTestContext(middleware.WithUser(context.Background(), user), upd)

	require.NoError(t, h.Handle(c))
	require.Equal(t, int32(0), atomic.LoadInt32(&rec.calls))
}

func TestTextHandler_NoUserInContext_NoOp(t *testing.T) {
	rec := &chatFlowRecorder{}
	h := tgh.NewTextHandler(uuid.New(), rec.handler())

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 1,
			Text:      "hi",
			From:      &tgbotapi.User{ID: 99},
			Chat:      &tgbotapi.Chat{ID: 99},
		},
	}
	c := tgbot.NewTestContext(context.Background(), upd)

	require.NoError(t, h.Handle(c))
	require.Equal(t, int32(0), atomic.LoadInt32(&rec.calls))
}
