package test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/middleware"
	"github.com/kotovconst/rollton/bot/pkg/tgbot"
)

type fakeUserSvc struct {
	user       domain.User
	err        error
	called     bool
	calledWith domain.User
}

func (f *fakeUserSvc) EnsureRegistered(_ context.Context, in domain.User) (domain.User, error) {
	f.called = true
	f.calledWith = in
	return f.user, f.err
}

func newCtx(upd tgbotapi.Update) *tgbot.Context {
	return tgbot.NewTestContext(context.Background(), upd)
}

var quietLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))

func TestEnsureUserRegistered_NoSender_PassesThrough(t *testing.T) {
	svc := &fakeUserSvc{}
	mw := middleware.EnsureUserRegistered(svc, quietLog)

	upd := tgbotapi.Update{} // no Message, no CallbackQuery -> SentFrom returns nil
	c := newCtx(upd)

	nextCalled := false
	handler := mw(func(*tgbot.Context) error { nextCalled = true; return nil })
	require.NoError(t, handler(c))
	require.True(t, nextCalled)
	require.False(t, svc.called)

	_, ok := middleware.UserFromContext(c.Ctx())
	require.False(t, ok)
}

func TestEnsureUserRegistered_Success_PutsUserInContext(t *testing.T) {
	stored := domain.User{ID: uuid.New(), TelegramID: 7, FirstName: "Bob"}
	svc := &fakeUserSvc{user: stored}
	mw := middleware.EnsureUserRegistered(svc, quietLog)

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 7, FirstName: "Bob"},
			Chat: &tgbotapi.Chat{ID: 7},
		},
	}
	c := newCtx(upd)

	handler := mw(func(c *tgbot.Context) error {
		u, ok := middleware.UserFromContext(c.Ctx())
		require.True(t, ok)
		require.Equal(t, stored.ID, u.ID)
		return nil
	})
	require.NoError(t, handler(c))
	require.True(t, svc.called)
	require.Equal(t, int64(7), svc.calledWith.TelegramID)
}

func TestEnsureUserRegistered_ServiceError_NextStillCalled(t *testing.T) {
	svc := &fakeUserSvc{err: errors.New("db down")}
	mw := middleware.EnsureUserRegistered(svc, quietLog)

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 9, FirstName: "Carol"},
			Chat: &tgbotapi.Chat{ID: 9},
		},
	}
	c := newCtx(upd)

	nextCalled := false
	handler := mw(func(c *tgbot.Context) error {
		nextCalled = true
		_, ok := middleware.UserFromContext(c.Ctx())
		require.False(t, ok, "no user in ctx when registration fails")
		return nil
	})
	require.NoError(t, handler(c))
	require.True(t, nextCalled)
}
