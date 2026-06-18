// Package usecase orchestrates services to complete one business action.
// Usecases own logging, error classification, and (where appropriate) DB
// transactions. Services beneath them stay quiet and single-purpose.
package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// GenericErrorReply is the user-facing message when the LLM call fails.
// The text never varies by sentinel — operator-facing detail goes to logs.
const GenericErrorReply = "Something went wrong, try again in a moment."

// ChatFlowDeps bundles the services + clients the chat-flow usecase needs.
type ChatFlowDeps struct {
	Chats      ports.ChatService
	Messages   ports.MessageService
	Contexts   ports.ContextService
	Characters ports.CharacterService
	OR         ports.OpenRouterClient
	HistoryN   int32
	Log        *slog.Logger
}

// NewChatFlowHandler returns a function that runs one user turn end-to-end:
// persist user msg → build prompt → call OpenRouter → send + persist reply.
// All logging happens inside the returned function.
//
// Chat-flow is deliberately non-transactional: an open tx held across the
// OpenRouter HTTP call would hold a DB connection for the full LLM duration,
// limiting concurrent throughput. Idempotency is delegated to the partial
// unique index on (chat_id, telegram_message_id).
func NewChatFlowHandler(deps ChatFlowDeps) ports.ChatFlowHandlerFunc {
	log := deps.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	}

	return func(
		ctx context.Context,
		user domain.User,
		characterID uuid.UUID,
		text string,
		tgUserMessageID int64,
		reply ports.ReplyFunc,
	) error {
		snap, err := resolveSnapshot(ctx, deps, user.ID, characterID)
		if err != nil {
			return err
		}

		userMsg, insertErr := deps.Messages.InsertUserIdempotent(ctx, snap.ChatID, text, tgUserMessageID)
		if insertErr != nil && !errors.Is(insertErr, pgx.ErrNoRows) {
			return insertErr
		}
		if errors.Is(insertErr, pgx.ErrNoRows) {
			orig, getErr := deps.Messages.GetUserByTGID(ctx, snap.ChatID, tgUserMessageID)
			if getErr != nil {
				return getErr
			}
			exists, existErr := deps.Messages.AssistantReplyExistsAfter(ctx, snap.ChatID, orig.CreatedAt)
			if existErr != nil {
				return existErr
			}
			if exists {
				log.Debug("chat_flow.duplicate_skip", "chat_id", snap.ChatID)
				return nil
			}
			// crash-before-reply path: fall through to LLM call.
			_ = userMsg
		}

		hist, err := deps.Messages.ListRecent(ctx, snap.ChatID, deps.HistoryN)
		if err != nil {
			return err
		}

		resp, callErr := deps.OR.Complete(ctx, openrouter.ChatRequest{
			Model:       snap.ModelName,
			Temperature: snap.Temperature,
			TopP:        snap.TopP,
			MaxTokens:   ptrInt32ToInt(snap.MaxTokens),
			Messages:    buildMessages(snap, hist),
		})
		if callErr != nil {
			if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) {
				log.Warn("chat_flow.cancelled",
					"chat_id", snap.ChatID, "character_slug", snap.CharacterSlug, "err", callErr.Error())
				return nil
			}
			log.Error("chat_flow.failed",
				"chat_id", snap.ChatID, "character_slug", snap.CharacterSlug,
				"err_class", classifyOpenRouterErr(callErr), "err", callErr.Error())
			if _, rerr := reply(GenericErrorReply); rerr != nil {
				log.Error("chat_flow.tg_send_failed", "chat_id", snap.ChatID, "err", rerr.Error())
			}
			return nil
		}

		firstChunkID, sendErr := reply(resp.Reply)
		if sendErr != nil {
			log.Error("chat_flow.tg_send_failed", "chat_id", snap.ChatID, "err", sendErr.Error())
			return nil
		}

		// Known trade-off: if InsertAssistant fails the user has already seen
		// the reply but no assistant row is persisted. A subsequent redelivery
		// will see no assistant reply via AssistantReplyExistsAfter and re-call
		// the LLM, producing a duplicate. Acceptable: TG redelivery without
		// successful ack is rare and a duplicate reply is less bad than a
		// missing one.
		if err := deps.Messages.InsertAssistant(ctx, ports.InsertAssistantArgs{
			ChatID:            snap.ChatID,
			Content:           resp.Reply,
			TelegramMessageID: firstChunkID,
			Model:             resp.Model,
			TokensIn:          int32(resp.TokensIn),
			TokensOut:         int32(resp.TokensOut),
		}); err != nil {
			log.Error("chat_flow.assistant_insert_failed", "chat_id", snap.ChatID, "err", err.Error())
			return nil
		}
		if err := deps.Chats.TouchUpdatedAt(ctx, snap.ChatID); err != nil {
			log.Error("chat_flow.touch_failed", "chat_id", snap.ChatID, "err", err.Error())
			return nil
		}
		log.Info("chat_flow.complete",
			"chat_id", snap.ChatID, "character_slug", snap.CharacterSlug,
			"model", resp.Model, "tokens_in", resp.TokensIn, "tokens_out", resp.TokensOut)
		return nil
	}
}

func resolveSnapshot(ctx context.Context, deps ChatFlowDeps, userID, characterID uuid.UUID) (domain.ChatFlowSnapshot, error) {
	snap, err := deps.Chats.GetMostRecentForUserCharacter(ctx, userID, characterID)
	if err == nil {
		return snap, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ChatFlowSnapshot{}, err
	}
	char, err := deps.Characters.GetByID(ctx, characterID)
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	defCtx, err := deps.Contexts.GetDefaultWithModelForCharacter(ctx, characterID)
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	chatID, err := deps.Chats.Create(ctx, userID, defCtx.ContextID)
	if err != nil {
		return domain.ChatFlowSnapshot{}, err
	}
	return domain.ChatFlowSnapshot{
		ChatID:              chatID,
		ContextID:           defCtx.ContextID,
		CharacterID:         characterID,
		CharacterSlug:       char.Slug,
		CharacterBasePrompt: char.BasePrompt,
		ContextPrompt:       defCtx.ContextPrompt,
		ModelName:           defCtx.ModelName,
		Temperature:         defCtx.Temperature,
		TopP:                defCtx.TopP,
		MaxTokens:           defCtx.MaxTokens,
	}, nil
}

func buildMessages(snap domain.ChatFlowSnapshot, hist []domain.RecentMessage) []openrouter.Message {
	out := make([]openrouter.Message, 0, 1+len(hist))
	out = append(out, openrouter.Message{
		Role:    openrouter.RoleSystem,
		Content: snap.CharacterBasePrompt + "\n\n" + snap.ContextPrompt,
	})
	// hist is newest-first from the service; reverse to chronological.
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
