package test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	tgh "github.com/kotovconst/rollton/bot/internal/bots/rolltonchatbot/handlers/telegram"
	"github.com/kotovconst/rollton/bot/internal/core/domain"
)

func TestStartHandler_ReplyText_WithUser(t *testing.T) {
	h := tgh.NewStartHandler()
	u := &domain.User{ID: uuid.New(), TelegramID: 1, FirstName: "Alice"}
	require.Contains(t, h.ReplyTextFor(u), "Alice")
	require.True(t, strings.Contains(strings.ToLower(h.ReplyTextFor(u)), "welcome"))
}

func TestStartHandler_ReplyText_FallsBackWithoutUser(t *testing.T) {
	h := tgh.NewStartHandler()
	require.Contains(t, h.ReplyTextFor(nil), "Welcome")
}
