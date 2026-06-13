package test

import (
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

func msgFrom(id int64) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: id, FirstName: "X"},
			Chat: &tgbotapi.Chat{ID: id},
		},
	}
}

func TestAllowOnlyUserIDs_AllowsListedUser(t *testing.T) {
	mw := middleware.AllowOnlyUserIDs([]int64{100, 200}, quietLog)
	c := tgbot.NewTestContext(context.Background(), msgFrom(100))

	called := false
	require.NoError(t, mw(func(*tgbot.Context) error { called = true; return nil })(c))
	require.True(t, called)
}

func TestAllowOnlyUserIDs_RejectsUnknownUser(t *testing.T) {
	mw := middleware.AllowOnlyUserIDs([]int64{100}, quietLog)
	c := tgbot.NewTestContext(context.Background(), msgFrom(999))

	called := false
	require.NoError(t, mw(func(*tgbot.Context) error { called = true; return nil })(c))
	require.False(t, called)
}

func TestAllowOnlyUserIDs_EmptyListRejectsAll(t *testing.T) {
	mw := middleware.AllowOnlyUserIDs(nil, quietLog)
	c := tgbot.NewTestContext(context.Background(), msgFrom(100))

	called := false
	require.NoError(t, mw(func(*tgbot.Context) error { called = true; return nil })(c))
	require.False(t, called)
}

func TestAllowOnlyUserIDs_NoSender_DropsSilently(t *testing.T) {
	mw := middleware.AllowOnlyUserIDs([]int64{100}, quietLog)
	c := tgbot.NewTestContext(context.Background(), tgbotapi.Update{})

	called := false
	require.NoError(t, mw(func(*tgbot.Context) error { called = true; return nil })(c))
	require.False(t, called)
}
