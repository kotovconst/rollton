package tgbot

import (
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

func TestRouter_DispatchesCommand(t *testing.T) {
	r := NewRouter()
	called := ""
	r.Handle("start", func(c *Context) error { called = "start"; return nil })

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/start",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
			Chat:     &tgbotapi.Chat{ID: 1},
		},
	}
	c := newContext(context.Background(), nil, upd)

	require.NoError(t, r.Dispatch(c))
	require.Equal(t, "start", called)
}

func TestRouter_DispatchesCallback(t *testing.T) {
	r := NewRouter()
	called := ""
	r.HandleCallback("yes", func(c *Context) error { called = "yes"; return nil })

	upd := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{Data: "yes"},
	}
	c := newContext(context.Background(), nil, upd)

	require.NoError(t, r.Dispatch(c))
	require.Equal(t, "yes", called)
}

func TestRouter_FallbackForUnknown(t *testing.T) {
	r := NewRouter()
	called := false
	r.HandleDefault(func(c *Context) error { called = true; return nil })

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{Text: "hi", Chat: &tgbotapi.Chat{ID: 1}},
	}
	c := newContext(context.Background(), nil, upd)

	require.NoError(t, r.Dispatch(c))
	require.True(t, called)
}
