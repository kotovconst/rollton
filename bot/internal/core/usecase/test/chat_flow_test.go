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
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/internal/core/usecase"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// --- fakes ---

type fakeChatSvc struct {
	mu          sync.Mutex
	snap        domain.ChatFlowSnapshot
	getErr      error
	createID    uuid.UUID
	createErr   error
	touchErr    error
	getCalls    int32
	createCalls int32
	touchCalls  int32
}

func (f *fakeChatSvc) GetMostRecentForUserCharacter(_ context.Context, _, _ uuid.UUID) (domain.ChatFlowSnapshot, error) {
	atomic.AddInt32(&f.getCalls, 1)
	return f.snap, f.getErr
}

func (f *fakeChatSvc) Create(_ context.Context, _, _ uuid.UUID) (uuid.UUID, error) {
	atomic.AddInt32(&f.createCalls, 1)
	return f.createID, f.createErr
}

func (f *fakeChatSvc) TouchUpdatedAt(_ context.Context, _ uuid.UUID) error {
	atomic.AddInt32(&f.touchCalls, 1)
	return f.touchErr
}

type fakeMsgSvc struct {
	mu                   sync.Mutex
	insertUser           domain.UserMsgRecord
	insertUserErr        error
	getUser              domain.UserMsgRecord
	getUserErr           error
	assistantExists      bool
	assistantExistsErr   error
	listRecent           []domain.RecentMessage
	listRecentErr        error
	insertAssistantErr   error
	insertAssistantArgs  ports.InsertAssistantArgs
	insertUserCalls      int32
	getUserCalls         int32
	assistantExistsCalls int32
	listRecentCalls      int32
	insertAssistantCalls int32
	listRecentLastLimit  int32
}

func (f *fakeMsgSvc) InsertUserIdempotent(_ context.Context, _ uuid.UUID, _ string, _ int64) (domain.UserMsgRecord, error) {
	atomic.AddInt32(&f.insertUserCalls, 1)
	return f.insertUser, f.insertUserErr
}

func (f *fakeMsgSvc) GetUserByTGID(_ context.Context, _ uuid.UUID, _ int64) (domain.UserMsgRecord, error) {
	atomic.AddInt32(&f.getUserCalls, 1)
	return f.getUser, f.getUserErr
}

func (f *fakeMsgSvc) AssistantReplyExistsAfter(_ context.Context, _ uuid.UUID, _ time.Time) (bool, error) {
	atomic.AddInt32(&f.assistantExistsCalls, 1)
	return f.assistantExists, f.assistantExistsErr
}

func (f *fakeMsgSvc) ListRecent(_ context.Context, _ uuid.UUID, limit int32) ([]domain.RecentMessage, error) {
	atomic.AddInt32(&f.listRecentCalls, 1)
	atomic.StoreInt32(&f.listRecentLastLimit, limit)
	return f.listRecent, f.listRecentErr
}

func (f *fakeMsgSvc) InsertAssistant(_ context.Context, args ports.InsertAssistantArgs) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	atomic.AddInt32(&f.insertAssistantCalls, 1)
	f.insertAssistantArgs = args
	return f.insertAssistantErr
}

type fakeCtxSvc struct {
	def   domain.DefaultContextForChat
	err   error
	calls int32
}

func (f *fakeCtxSvc) GetDefaultWithModelForCharacter(_ context.Context, _ uuid.UUID) (domain.DefaultContextForChat, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.def, f.err
}

type fakeCharSvc struct {
	char     domain.Character
	err      error
	getCalls int32
}

func (f *fakeCharSvc) ListActive(_ context.Context) ([]domain.Character, error) { return nil, nil }
func (f *fakeCharSvc) GetByID(_ context.Context, _ uuid.UUID) (domain.Character, error) {
	atomic.AddInt32(&f.getCalls, 1)
	return f.char, f.err
}

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

// --- helpers ---

func sampleUser() domain.User {
	return domain.User{ID: uuid.New(), TelegramID: 42, FirstName: "Test"}
}

func sampleSnap(chatID, ctxID, charID uuid.UUID) domain.ChatFlowSnapshot {
	temp := 0.7
	max := int32(256)
	return domain.ChatFlowSnapshot{
		ChatID:              chatID,
		ContextID:           ctxID,
		CharacterID:         charID,
		CharacterSlug:       "snoop-dogg",
		CharacterBasePrompt: "You are Snoop.",
		ContextPrompt:       "Setting: studio.",
		ModelName:           "anthropic/claude-haiku-4.5",
		Temperature:         &temp,
		MaxTokens:           &max,
	}
}

func newDeps(chats *fakeChatSvc, msgs *fakeMsgSvc, ctxs *fakeCtxSvc, chars *fakeCharSvc, or *fakeOR, historyN int32) usecase.ChatFlowDeps {
	return usecase.ChatFlowDeps{
		Chats:      chats,
		Messages:   msgs,
		Contexts:   ctxs,
		Characters: chars,
		OR:         or,
		HistoryN:   historyN,
		Log:        nil,
	}
}

// --- tests ---

func TestChatFlow_HappyPath_ExistingChat(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	snap := sampleSnap(chatID, ctxID, charID)

	chats := &fakeChatSvc{snap: snap}
	msgs := &fakeMsgSvc{
		insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: time.Now()},
		listRecent: []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: time.Now()}},
	}
	or := &fakeOR{resp: openrouter.ChatResponse{
		Model: "anthropic/claude-haiku-4.5", Reply: "hello back",
		TokensIn: 10, TokensOut: 5, FinishReason: "stop",
	}}
	rep := &fakeReply{returnID: 7777}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(1), atomic.LoadInt32(&or.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&rep.calls))
	require.Equal(t, "hello back", rep.lastText)
	require.Equal(t, int32(1), atomic.LoadInt32(&msgs.insertAssistantCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&chats.touchCalls))

	require.GreaterOrEqual(t, len(or.lastReq.Messages), 2)
	require.Equal(t, openrouter.RoleSystem, or.lastReq.Messages[0].Role)
	require.Contains(t, or.lastReq.Messages[0].Content, "You are Snoop.")
	require.Contains(t, or.lastReq.Messages[0].Content, "Setting: studio.")
}

func TestChatFlow_NewChat_AutoCreatesWithDefaultContext(t *testing.T) {
	charID := uuid.New()
	chatID := uuid.New()
	ctxID := uuid.New()
	now := time.Now()

	chats := &fakeChatSvc{getErr: pgx.ErrNoRows, createID: chatID}
	chars := &fakeCharSvc{char: domain.Character{ID: charID, Slug: "snoop-dogg", BasePrompt: "You are Snoop."}}
	ctxs := &fakeCtxSvc{def: domain.DefaultContextForChat{
		ContextID: ctxID, ContextPrompt: "Setting: studio.", ModelName: "anthropic/claude-haiku-4.5",
	}}
	msgs := &fakeMsgSvc{
		insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: now},
		listRecent: []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: now}},
	}
	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "yo"}}
	rep := &fakeReply{returnID: 7}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, ctxs, chars, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(1), atomic.LoadInt32(&chars.getCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&ctxs.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&chats.createCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&or.calls))
}

func TestChatFlow_Duplicate_AlreadyReplied_Skips(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	prevCreated := time.Now().Add(-1 * time.Minute)

	chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
	msgs := &fakeMsgSvc{
		insertUserErr:   pgx.ErrNoRows,
		getUser:         domain.UserMsgRecord{ID: uuid.New(), CreatedAt: prevCreated},
		assistantExists: true,
	}
	or := &fakeOR{}
	rep := &fakeReply{}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(0), atomic.LoadInt32(&or.calls), "no LLM call on already-replied duplicate")
	require.Equal(t, int32(0), atomic.LoadInt32(&rep.calls))
	require.Equal(t, int32(0), atomic.LoadInt32(&msgs.insertAssistantCalls))
}

func TestChatFlow_Duplicate_NoReply_RetriesLLM(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	prevCreated := time.Now().Add(-30 * time.Second)

	chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
	msgs := &fakeMsgSvc{
		insertUserErr:   pgx.ErrNoRows,
		getUser:         domain.UserMsgRecord{ID: uuid.New(), CreatedAt: prevCreated},
		assistantExists: false,
		listRecent:      []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: prevCreated}},
	}
	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "yo"}}
	rep := &fakeReply{returnID: 7}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(1), atomic.LoadInt32(&or.calls))
	require.Equal(t, int32(1), atomic.LoadInt32(&msgs.insertAssistantCalls))
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
			charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
			now := time.Now()

			chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
			msgs := &fakeMsgSvc{
				insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: now},
				listRecent: []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: now}},
			}
			or := &fakeOR{err: c.err}
			rep := &fakeReply{returnID: 7}

			handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
			require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

			require.Equal(t, int32(1), atomic.LoadInt32(&rep.calls))
			require.Equal(t, usecase.GenericErrorReply, rep.lastText)
			require.Equal(t, int32(0), atomic.LoadInt32(&msgs.insertAssistantCalls))
		})
	}
}

func TestChatFlow_PromptShape_HistoryReversedToChronological(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
	// Service returns DESC (newest-first).
	msgs := &fakeMsgSvc{
		insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: now},
		listRecent: []domain.RecentMessage{
			{Role: "user", Content: "hi", CreatedAt: now},
			{Role: "user", Content: "older user", CreatedAt: now.Add(-2 * time.Second)},
			{Role: "assistant", Content: "older asst", CreatedAt: now.Add(-3 * time.Second)},
		},
	}
	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "ok"}}
	rep := &fakeReply{returnID: 7}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	m := or.lastReq.Messages
	require.GreaterOrEqual(t, len(m), 4)
	require.Equal(t, openrouter.RoleSystem, m[0].Role)
	require.Equal(t, openrouter.RoleAssistant, m[1].Role)
	require.Equal(t, "older asst", m[1].Content)
	require.Equal(t, openrouter.RoleUser, m[2].Role)
	require.Equal(t, "older user", m[2].Content)
	require.Equal(t, openrouter.RoleUser, m[3].Role)
	require.Equal(t, "hi", m[3].Content)
}

func TestChatFlow_HistoryWindowLimit_PassedToService(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
	msgs := &fakeMsgSvc{
		insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: now},
		listRecent: []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: now}},
	}
	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "ok"}}
	rep := &fakeReply{returnID: 7}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 5))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(5), atomic.LoadInt32(&msgs.listRecentLastLimit))
}

func TestChatFlow_TGSendFails_NoAssistantInsert(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
	msgs := &fakeMsgSvc{
		insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: now},
		listRecent: []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: now}},
	}
	or := &fakeOR{resp: openrouter.ChatResponse{Model: "m", Reply: "ok"}}
	rep := &fakeReply{returnID: 7, returnErr: errors.New("tg send failed")}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(0), atomic.LoadInt32(&msgs.insertAssistantCalls), "no assistant row inserted when TG send fails")
}

func TestChatFlow_CtxCancelled_WarnAndReturnNil(t *testing.T) {
	charID, chatID, ctxID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	chats := &fakeChatSvc{snap: sampleSnap(chatID, ctxID, charID)}
	msgs := &fakeMsgSvc{
		insertUser: domain.UserMsgRecord{ID: uuid.New(), CreatedAt: now},
		listRecent: []domain.RecentMessage{{Role: "user", Content: "hi", CreatedAt: now}},
	}
	or := &fakeOR{err: context.Canceled}
	rep := &fakeReply{returnID: 7}

	handler := usecase.NewChatFlowHandler(newDeps(chats, msgs, &fakeCtxSvc{}, &fakeCharSvc{}, or, 20))
	require.NoError(t, handler(context.Background(), sampleUser(), charID, "hi", 999, rep.fn()))

	require.Equal(t, int32(0), atomic.LoadInt32(&rep.calls), "no user-facing reply on cancellation")
	require.Equal(t, int32(0), atomic.LoadInt32(&msgs.insertAssistantCalls))
}
