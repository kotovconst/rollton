package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

// GenericErrorReply is the user-facing message when the LLM call fails.
// Same text for every sentinel — operator-facing detail goes to logs.
const GenericErrorReply = "Something went wrong, try again in a moment."

type chatFlowService struct {
	q        *postgres.Queries
	or       ports.OpenRouterClient
	historyN int32
	log      *slog.Logger
}

// NewChatFlowService wires the service against any postgres.DBTX (live pool
// or pgxmock). log may be nil (discarded).
func NewChatFlowService(db postgres.DBTX, or ports.OpenRouterClient, historyN int32, log *slog.Logger) ports.ChatFlowService {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	}
	return &chatFlowService{
		q:        postgres.New(db),
		or:       or,
		historyN: historyN,
		log:      log,
	}
}

func (s *chatFlowService) Handle(
	ctx context.Context,
	user domain.User,
	characterID uuid.UUID,
	text string,
	tgUserMessageID int64,
	reply ports.ReplyFunc,
) error {
	snap, err := s.resolveSnapshot(ctx, user.ID, characterID)
	if err != nil {
		return err
	}

	_, insertErr := s.q.InsertUserMessageIdempotent(ctx, postgres.InsertUserMessageIdempotentParams{
		ChatID:            pgtype.UUID{Bytes: snap.ChatID, Valid: true},
		Content:           text,
		TelegramMessageID: pgtype.Int8{Int64: tgUserMessageID, Valid: true},
	})
	if insertErr != nil && !errors.Is(insertErr, pgx.ErrNoRows) {
		return insertErr
	}
	if errors.Is(insertErr, pgx.ErrNoRows) {
		orig, getErr := s.q.GetUserMessageByTelegramID(ctx, postgres.GetUserMessageByTelegramIDParams{
			ChatID:            pgtype.UUID{Bytes: snap.ChatID, Valid: true},
			TelegramMessageID: pgtype.Int8{Int64: tgUserMessageID, Valid: true},
		})
		if getErr != nil {
			return getErr
		}
		exists, existErr := s.q.AssistantReplyExistsAfter(ctx, postgres.AssistantReplyExistsAfterParams{
			ChatID:    pgtype.UUID{Bytes: snap.ChatID, Valid: true},
			CreatedAt: orig.CreatedAt,
		})
		if existErr != nil {
			return existErr
		}
		if exists {
			s.log.Debug("chat_flow.duplicate_skip", "chat_id", snap.ChatID)
			return nil
		}
	}

	hist, err := s.q.ListRecentMessages(ctx, postgres.ListRecentMessagesParams{
		ChatID: pgtype.UUID{Bytes: snap.ChatID, Valid: true},
		Limit:  s.historyN,
	})
	if err != nil {
		return err
	}

	resp, callErr := s.or.Complete(ctx, openrouter.ChatRequest{
		Model:       snap.ModelName,
		Temperature: snap.Temperature,
		TopP:        snap.TopP,
		MaxTokens:   ptrInt32ToInt(snap.MaxTokens),
		Messages:    buildMessages(snap, hist),
	})
	if callErr != nil {
		if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) {
			s.log.Warn("chat_flow.cancelled",
				"chat_id", snap.ChatID, "character_slug", snap.CharacterSlug, "err", callErr.Error())
			return nil
		}
		s.log.Error("chat_flow.failed",
			"chat_id", snap.ChatID, "character_slug", snap.CharacterSlug,
			"err_class", classifyOpenRouterErr(callErr), "err", callErr.Error())
		if _, rerr := reply(GenericErrorReply); rerr != nil {
			s.log.Error("chat_flow.tg_send_failed", "chat_id", snap.ChatID, "err", rerr.Error())
		}
		return nil
	}

	firstChunkID, sendErr := reply(resp.Reply)
	if sendErr != nil {
		s.log.Error("chat_flow.tg_send_failed", "chat_id", snap.ChatID, "err", sendErr.Error())
		return nil
	}
	// Known trade-off: if this insert fails, the user has already seen the
	// reply but no assistant row is persisted. A subsequent redelivery of the
	// same user message will see no assistant reply via AssistantReplyExistsAfter
	// and re-call the LLM, producing a second reply. Acceptable because TG
	// redelivery without a successful ack is rare and a duplicate reply is
	// less bad than a missing one.
	if err := s.q.InsertAssistantMessage(ctx, postgres.InsertAssistantMessageParams{
		ChatID:            pgtype.UUID{Bytes: snap.ChatID, Valid: true},
		Content:           resp.Reply,
		TelegramMessageID: pgtype.Int8{Int64: firstChunkID, Valid: true},
		LlmModel:          pgtype.Text{String: resp.Model, Valid: true},
		LlmTokensIn:       pgtype.Int4{Int32: int32(resp.TokensIn), Valid: true},
		LlmTokensOut:      pgtype.Int4{Int32: int32(resp.TokensOut), Valid: true},
	}); err != nil {
		s.log.Error("chat_flow.assistant_insert_failed", "chat_id", snap.ChatID, "err", err.Error())
		return nil
	}
	if err := s.q.TouchChatUpdatedAt(ctx, pgtype.UUID{Bytes: snap.ChatID, Valid: true}); err != nil {
		s.log.Error("chat_flow.touch_failed", "chat_id", snap.ChatID, "err", err.Error())
		return nil
	}
	s.log.Info("chat_flow.complete",
		"chat_id", snap.ChatID, "character_slug", snap.CharacterSlug,
		"model", resp.Model, "tokens_in", resp.TokensIn, "tokens_out", resp.TokensOut)
	return nil
}

func (s *chatFlowService) resolveSnapshot(ctx context.Context, userID, charID uuid.UUID) (domain.ChatFlowSnapshot, error) {
	row, err := s.q.GetMostRecentChatJoinedForUserCharacter(ctx, postgres.GetMostRecentChatJoinedForUserCharacterParams{
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		ID:     pgtype.UUID{Bytes: charID, Valid: true},
	})
	if err == nil {
		return domain.NewChatFlowSnapshotFromJoinedRow(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ChatFlowSnapshot{}, err
	}

	char, err := s.q.GetCharacterByID(ctx, pgtype.UUID{Bytes: charID, Valid: true})
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	defCtx, err := s.q.GetDefaultContextWithModelForCharacter(ctx, pgtype.UUID{Bytes: charID, Valid: true})
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	newChat, err := s.q.InsertChat(ctx, postgres.InsertChatParams{
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		ContextID: defCtx.ContextID,
	})
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	return domain.NewChatFlowSnapshotFromContext(
		uuid.UUID(newChat.ID.Bytes), charID, char.Slug, char.BasePrompt, defCtx,
	), nil
}

func buildMessages(snap domain.ChatFlowSnapshot, hist []postgres.ListRecentMessagesRow) []openrouter.Message {
	out := make([]openrouter.Message, 0, 1+len(hist))
	out = append(out, openrouter.Message{
		Role:    openrouter.RoleSystem,
		Content: snap.CharacterBasePrompt + "\n\n" + snap.ContextPrompt,
	})
	// hist is newest-first from the SQL ORDER BY DESC; reverse to chronological.
	for i := len(hist) - 1; i >= 0; i-- {
		role := mapRole(hist[i].Role)
		if role == "" {
			continue
		}
		out = append(out, openrouter.Message{Role: role, Content: hist[i].Content})
	}
	return out
}

func mapRole(r string) openrouter.Role {
	switch strings.ToLower(r) {
	case "user":
		return openrouter.RoleUser
	case "assistant":
		return openrouter.RoleAssistant
	}
	return ""
}

// classifyOpenRouterErr maps an OpenRouter call error to a domain enum.
func classifyOpenRouterErr(err error) domain.ChatFlowErrClass {
	switch {
	case errors.Is(err, openrouter.ErrInvalidAuth):
		return domain.ChatFlowErrInvalidAuth
	case errors.Is(err, openrouter.ErrInsufficientCredits):
		return domain.ChatFlowErrInsufficientCredits
	case errors.Is(err, openrouter.ErrRateLimited):
		return domain.ChatFlowErrRateLimited
	case errors.Is(err, openrouter.ErrUpstream):
		return domain.ChatFlowErrUpstream
	default:
		return domain.ChatFlowErrUnknown
	}
}

func ptrInt32ToInt(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
