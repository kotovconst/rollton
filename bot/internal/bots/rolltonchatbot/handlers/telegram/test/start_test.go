package test

import (
	"testing"

	tgh "github.com/kotovconst/rollton/bot/internal/bots/rolltonchatbot/handlers/telegram"
	"github.com/stretchr/testify/require"
)

func TestStartHandler_ReplyText(t *testing.T) {
	h := tgh.NewStartHandler()
	require.Contains(t, h.ReplyText(), "Welcome")
}
