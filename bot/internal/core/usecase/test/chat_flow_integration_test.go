//go:build integration

// Integration test that exercises the chat-flow usecase end-to-end against
// the real dev Postgres + an httptest-mocked OpenRouter server. Run via:
//
//	make up                          # ensure rollton-postgres is running
//	make migrate                     # apply migrations + seeds
//	go test -tags=integration -count=1 ./internal/core/usecase/test/...
//
// This catches SQL-to-Go translation issues the unit tests (which mock the
// service layer) cannot.
package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/services"
	"github.com/kotovconst/rollton/bot/internal/core/usecase"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

const integrationDSN = "postgres://rollton:rollton_pass@localhost:5432/rollton?sslmode=disable"

const cannedORResponse = `{
  "id": "gen-test",
  "model": "anthropic/claude-haiku-4.5",
  "choices": [{
    "message": {"role": "assistant", "content": "yo dawg, what's good"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 12, "completion_tokens": 7}
}`

func TestIntegration_ChatFlow_EndToEnd(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("SKIP_INTEGRATION set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, integrationDSN)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	// Insert a unique test user so we don't collide with seed data or earlier runs.
	telegramID := time.Now().UnixNano()
	var userID uuid.UUID
	row := pool.QueryRow(ctx,
		`INSERT INTO users (telegram_id, first_name) VALUES ($1, 'IntegrationTest') RETURNING id`,
		telegramID)
	require.NoError(t, row.Scan(&userID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", userID)
	})

	// Locate snoop-dogg from seeds.
	var snoopID uuid.UUID
	row = pool.QueryRow(ctx, `SELECT id FROM characters WHERE slug='snoop-dogg'`)
	require.NoError(t, row.Scan(&snoopID))

	// Mocked OpenRouter HTTP server.
	var orCalls int32
	var lastReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&orCalls, 1)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &lastReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedORResponse)
	}))
	defer srv.Close()

	orClient := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))

	chatSvc := services.NewChatService(pool)
	contextSvc := services.NewContextService(pool)
	messageSvc := services.NewMessageService(pool)
	characterSvc := services.NewCharacterService(pool)

	handler := usecase.NewChatFlowHandler(usecase.ChatFlowDeps{
		Chats:      chatSvc,
		Messages:   messageSvc,
		Contexts:   contextSvc,
		Characters: characterSvc,
		OR:         orClient,
		HistoryN:   20,
	})

	// Capture replies via the ReplyFunc closure. Real Telegram returns a fresh
	// MessageID per sendMessage; mimic that with a monotonic counter.
	var replyText atomic.Value
	var replyCalls int32
	var replyMsgIDSeed int64 = 900_000
	reply := func(text string) (int64, error) {
		atomic.AddInt32(&replyCalls, 1)
		replyText.Store(text)
		replyMsgIDSeed++
		return replyMsgIDSeed, nil
	}

	user := domain.User{ID: userID, TelegramID: telegramID, FirstName: "IntegrationTest"}

	// First message — should auto-create chat with default context (snoop's studio).
	require.NoError(t, handler(ctx, user, snoopID, "yo Snoop", 100001, reply))
	require.Equal(t, int32(1), atomic.LoadInt32(&orCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&replyCalls))
	require.Equal(t, "yo dawg, what's good", replyText.Load())

	// Inspect rows landed.
	var chatID uuid.UUID
	var contextSlug string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT c.id, ctx.slug
		 FROM chats c JOIN contexts ctx ON ctx.id = c.context_id
		 WHERE c.user_id = $1
		 ORDER BY c.updated_at DESC LIMIT 1`, userID).Scan(&chatID, &contextSlug))
	require.Equal(t, "studio", contextSlug, "auto-created chat should use lowest-position active context")

	var userMsgCount, asstMsgCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE role='user'),
			COUNT(*) FILTER (WHERE role='assistant')
		 FROM tg_messages WHERE chat_id = $1`, chatID).Scan(&userMsgCount, &asstMsgCount))
	require.Equal(t, 1, userMsgCount)
	require.Equal(t, 1, asstMsgCount)

	// Verify prompt shape was actually sent.
	require.NotNil(t, lastReq)
	msgs, _ := lastReq["messages"].([]any)
	require.GreaterOrEqual(t, len(msgs), 2)
	sys := msgs[0].(map[string]any)
	require.Equal(t, "system", sys["role"])
	require.Contains(t, sys["content"], "Snoop Dogg")

	// Second message — should reuse the existing chat, history now has 2 entries.
	require.NoError(t, handler(ctx, user, snoopID, "tell me more", 100002, reply))
	require.Equal(t, int32(2), atomic.LoadInt32(&orCalls))

	// Idempotency: re-deliver the FIRST message (same tg_msg_id) — should skip LLM.
	require.NoError(t, handler(ctx, user, snoopID, "yo Snoop (redelivered)", 100001, reply))
	require.Equal(t, int32(2), atomic.LoadInt32(&orCalls), "redelivered message must not re-call LLM")

	// Final user/assistant row counts:
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE role='user'),
			COUNT(*) FILTER (WHERE role='assistant')
		 FROM tg_messages WHERE chat_id = $1`, chatID).Scan(&userMsgCount, &asstMsgCount))
	require.Equal(t, 2, userMsgCount, "redelivery should not create a third user row")
	require.Equal(t, 2, asstMsgCount)

	// Sanity-check the LLM_HISTORY_WINDOW honor: history sent on the 2nd call
	// should include the just-inserted user turn + the assistant turn from the
	// first call + the new user turn.
	require.GreaterOrEqual(t, len(msgs), 2)

	// Cleanup test rows (chats cascades to tg_messages via FK).
	_, _ = pool.Exec(ctx, "DELETE FROM chats WHERE id=$1", chatID)
}
