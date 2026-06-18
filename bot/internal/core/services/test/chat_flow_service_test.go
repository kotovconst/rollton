package test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/internal/core/services"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// --- fakes ---

type fakeOR struct {
	mu      sync.Mutex
	calls   int32
	lastReq openrouter.ChatRequest
	resp    openrouter.ChatResponse
	err     error
}

func (f *fakeOR) Complete(_ context.Context, req openrouter.ChatRequest) (openrouter.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	atomic.AddInt32(&f.calls, 1)
	f.lastReq = req
	return f.resp, f.err
}

type fakeReply struct {
	mu        sync.Mutex
	calls     int32
	lastText  string
	returnID  int64
	returnErr error
}

func (f *fakeReply) fn() ports.ReplyFunc {
	return func(text string) (int64, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		atomic.AddInt32(&f.calls, 1)
		f.lastText = text
		return f.returnID, f.returnErr
	}
}

// --- column lists (verify field order matches sqlc-generated row struct) ---

func joinedCols() []string {
	return []string{
		"chat_id", "chat_status", "chat_summary", "chat_updated_at",
		"context_id", "context_slug", "context_name", "context_prompt", "is_age_restricted",
		"character_id", "character_slug", "character_name", "character_base_prompt",
		"model_config_id", "model_slug", "model_name",
		"temperature", "top_p", "max_tokens",
	}
}

func defCtxCols() []string {
	return []string{
		"context_id", "context_slug", "context_name", "context_prompt", "is_age_restricted",
		"model_config_id", "model_slug", "model_name",
		"temperature", "top_p", "max_tokens",
	}
}

func joinedRow(chatID, ctxID, charID, mcID uuid.UUID) []any {
	now := time.Now()
	return []any{
		pgtype.UUID{Bytes: chatID, Valid: true}, "active", pgtype.Text{},
		pgtype.Timestamptz{Time: now, Valid: true},
		pgtype.UUID{Bytes: ctxID, Valid: true}, "studio", "In the Studio", "Setting: studio.", false,
		pgtype.UUID{Bytes: charID, Valid: true}, "snoop-dogg", "Snoop Dogg", "You are Snoop.",
		pgtype.UUID{Bytes: mcID, Valid: true}, "fast", "anthropic/claude-haiku-4.5",
		pgtype.Float8{Float64: 0.7, Valid: true}, pgtype.Float8{}, pgtype.Int4{Int32: 256, Valid: true},
	}
}

func defCtxRow(ctxID, mcID uuid.UUID) []any {
	return []any{
		pgtype.UUID{Bytes: ctxID, Valid: true}, "studio", "In the Studio", "Setting: studio.", false,
		pgtype.UUID{Bytes: mcID, Valid: true}, "fast", "anthropic/claude-haiku-4.5",
		pgtype.Float8{Float64: 0.7, Valid: true}, pgtype.Float8{}, pgtype.Int4{Int32: 256, Valid: true},
	}
}

func sampleUser() domain.User {
	return domain.User{ID: uuid.New(), TelegramID: 42, FirstName: "Test"}
}

// any2 is shorthand for two AnyArg matchers (used frequently for (chatID, arg) pairs).
func any2() []interface{} {
	return []interface{}{pgxmock.AnyArg(), pgxmock.AnyArg()}
}

// any3 is shorthand for three AnyArg matchers.
func any3() []interface{} {
	return []interface{}{pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()}
}

// --- tests ---

func TestChatFlow_HappyPath_ExistingChat(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	// GetMostRecentChatJoinedForUserCharacter(userID, charID)
	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
	// InsertUserMessageIdempotent(chatID, content, tgMsgID)
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}))
	// ListRecentMessages(chatID, limit)
	mock.ExpectQuery("SELECT role, content, created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
			AddRow("user", "hi", pgtype.Timestamptz{Time: now, Valid: true}))
	// InsertAssistantMessage(chatID, content, tgMsgID, model, tokIn, tokOut)
	mock.ExpectExec("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// TouchChatUpdatedAt(chatID)
	mock.ExpectExec("UPDATE chats SET updated_at").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	or := &fakeOR{resp: openrouter.ChatResponse{
		Model: "anthropic/claude-haiku-4.5", Reply: "hello back",
		TokensIn: 10, TokensOut: 5, FinishReason: "stop",
	}}
	rep := &fakeReply{returnID: 7777}

	svc := services.NewChatFlowService(mock, or, 20, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, int32(1), atomic.LoadInt32(&or.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&rep.calls))
	require.Equal(t, "hello back", rep.lastText)

	require.GreaterOrEqual(t, len(or.lastReq.Messages), 2)
	require.Equal(t, openrouter.RoleSystem, or.lastReq.Messages[0].Role)
	require.Contains(t, or.lastReq.Messages[0].Content, "You are Snoop.")
	require.Contains(t, or.lastReq.Messages[0].Content, "Setting: studio.")
}

func TestChatFlow_NewChat_AutoCreatesWithDefaultContext(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	// GetMostRecentChatJoinedForUserCharacter → no rows
	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)
	// GetCharacterByID(charID)
	mock.ExpectQuery("FROM characters").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "slug", "base_prompt"}).
			AddRow(pgtype.UUID{Bytes: charID, Valid: true}, "snoop-dogg", "You are Snoop."))
	// GetDefaultContextWithModelForCharacter(charID)
	mock.ExpectQuery("FROM contexts ctx").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(defCtxCols()).AddRow(defCtxRow(ctxID, mcID)...))
	// InsertChat(userID, contextID)
	mock.ExpectQuery("INSERT INTO chats").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "status", "summary", "updated_at"}).
			AddRow(pgtype.UUID{Bytes: chatID, Valid: true}, "active", pgtype.Text{},
				pgtype.Timestamptz{Time: now, Valid: true}))
	// InsertUserMessageIdempotent(chatID, content, tgMsgID)
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}))
	// ListRecentMessages(chatID, limit)
	mock.ExpectQuery("SELECT role, content, created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
			AddRow("user", "hi", pgtype.Timestamptz{Time: now, Valid: true}))
	// InsertAssistantMessage
	mock.ExpectExec("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// TouchChatUpdatedAt
	mock.ExpectExec("UPDATE chats SET updated_at").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "yo"}}
	rep := &fakeReply{returnID: 7}

	svc := services.NewChatFlowService(mock, or, 20, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatFlow_Duplicate_AlreadyReplied_Skips(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	prevCreated := time.Now().Add(-1 * time.Minute)

	// GetMostRecentChatJoinedForUserCharacter
	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
	// InsertUserMessageIdempotent → conflict (ErrNoRows sentinel)
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)
	// GetUserMessageByTelegramID(chatID, tgMsgID)
	mock.ExpectQuery("FROM tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: prevCreated, Valid: true}))
	// AssistantReplyExistsAfter(chatID, createdAt) → true
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"reply_exists"}).AddRow(true))

	or := &fakeOR{}
	rep := &fakeReply{}
	svc := services.NewChatFlowService(mock, or, 20, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, int32(0), atomic.LoadInt32(&or.calls))
	require.Equal(t, int32(0), atomic.LoadInt32(&rep.calls))
}

func TestChatFlow_Duplicate_NoReply_RetriesLLM(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	prevCreated := time.Now().Add(-30 * time.Second)

	// GetMostRecentChatJoinedForUserCharacter
	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
	// InsertUserMessageIdempotent → conflict
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(pgx.ErrNoRows)
	// GetUserMessageByTelegramID
	mock.ExpectQuery("FROM tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: prevCreated, Valid: true}))
	// AssistantReplyExistsAfter → false (no prior reply)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"reply_exists"}).AddRow(false))
	// ListRecentMessages
	mock.ExpectQuery("SELECT role, content, created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
			AddRow("user", "hi", pgtype.Timestamptz{Time: prevCreated, Valid: true}))
	// InsertAssistantMessage
	mock.ExpectExec("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// TouchChatUpdatedAt
	mock.ExpectExec("UPDATE chats SET updated_at").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "yo"}}
	rep := &fakeReply{returnID: 7}
	svc := services.NewChatFlowService(mock, or, 20, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, int32(1), atomic.LoadInt32(&or.calls))
}

func TestChatFlow_LLMErrors_AllRepliedGenerically(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"ErrInvalidAuth", openrouter.ErrInvalidAuth},
		{"ErrInsufficientCredits", openrouter.ErrInsufficientCredits},
		{"ErrRateLimited", openrouter.ErrRateLimited},
		{"ErrUpstream", openrouter.ErrUpstream},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()

			user := sampleUser()
			charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			now := time.Now()

			mock.ExpectQuery("FROM chats c").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
			mock.ExpectQuery("INSERT INTO tg_messages").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
					AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}))
			mock.ExpectQuery("SELECT role, content, created_at").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
					AddRow("user", "hi", pgtype.Timestamptz{Time: now, Valid: true}))

			or := &fakeOR{err: c.err}
			rep := &fakeReply{returnID: 7}
			svc := services.NewChatFlowService(mock, or, 20, nil)
			require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
			require.NoError(t, mock.ExpectationsWereMet())
			require.Equal(t, int32(1), atomic.LoadInt32(&rep.calls))
			require.Equal(t, services.GenericErrorReply, rep.lastText)
		})
	}
}

func TestChatFlow_PromptShape_HistoryReversedToChronological(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}))
	// sqlc query orders DESC, so we feed newest-first here.
	mock.ExpectQuery("SELECT role, content, created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
			AddRow("user", "hi", pgtype.Timestamptz{Time: now, Valid: true}).
			AddRow("user", "older user", pgtype.Timestamptz{Time: now.Add(-2 * time.Second), Valid: true}).
			AddRow("assistant", "older asst", pgtype.Timestamptz{Time: now.Add(-3 * time.Second), Valid: true}))
	mock.ExpectExec("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE chats SET updated_at").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "ok"}}
	rep := &fakeReply{returnID: 7}
	svc := services.NewChatFlowService(mock, or, 20, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))

	msgs := or.lastReq.Messages
	require.GreaterOrEqual(t, len(msgs), 4)
	require.Equal(t, openrouter.RoleSystem, msgs[0].Role)
	require.Equal(t, openrouter.RoleAssistant, msgs[1].Role)
	require.Equal(t, "older asst", msgs[1].Content)
	require.Equal(t, openrouter.RoleUser, msgs[2].Role)
	require.Equal(t, "older user", msgs[2].Content)
	require.Equal(t, openrouter.RoleUser, msgs[3].Role)
	require.Equal(t, "hi", msgs[3].Content)
}

func TestChatFlow_HistoryWindowLimit_PassedToQuery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}))
	// Verify historyN=5 is passed as the limit argument (int32).
	mock.ExpectQuery("SELECT role, content, created_at").
		WithArgs(pgxmock.AnyArg(), int32(5)).
		WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
			AddRow("user", "hi", pgtype.Timestamptz{Time: now, Valid: true}))
	mock.ExpectExec("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE chats SET updated_at").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "ok"}}
	rep := &fakeReply{returnID: 7}
	svc := services.NewChatFlowService(mock, or, 5, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChatFlow_TGSendFails_NoAssistantInsert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	user := sampleUser()
	charID, chatID, ctxID, mcID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	mock.ExpectQuery("FROM chats c").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(joinedCols()).AddRow(joinedRow(chatID, ctxID, charID, mcID)...))
	mock.ExpectQuery("INSERT INTO tg_messages").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(pgtype.UUID{Bytes: uuid.New(), Valid: true}, pgtype.Timestamptz{Time: now, Valid: true}))
	mock.ExpectQuery("SELECT role, content, created_at").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"role", "content", "created_at"}).
			AddRow("user", "hi", pgtype.Timestamptz{Time: now, Valid: true}))
	// no ExpectExec for assistant insert — must NOT happen.

	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "ok"}}
	rep := &fakeReply{returnID: 7, returnErr: errors.New("tg send failed")}
	svc := services.NewChatFlowService(mock, or, 20, nil)
	require.NoError(t, svc.Handle(context.Background(), user, charID, "hi", 999, rep.fn()))
	require.NoError(t, mock.ExpectationsWereMet())
}
