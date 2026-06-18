package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
	"github.com/kotovconst/rollton/bot/pkg/sqlc/postgres"
)

type messageService struct {
	queries *postgres.Queries
}

func NewMessageService(db postgres.DBTX) ports.MessageService {
	return &messageService{queries: postgres.New(db)}
}

func (s *messageService) InsertUserIdempotent(ctx context.Context, chatID uuid.UUID, content string, tgMessageID int64) (domain.UserMsgRecord, error) {
	row, err := s.queries.InsertUserMessageIdempotent(ctx, postgres.InsertUserMessageIdempotentParams{
		ChatID:            pgtype.UUID{Bytes: chatID, Valid: true},
		Content:           content,
		TelegramMessageID: pgtype.Int8{Int64: tgMessageID, Valid: true},
	})
	if err != nil {
		return domain.UserMsgRecord{}, err
	}
	return domain.UserMsgRecord{ID: uuid.UUID(row.ID.Bytes), CreatedAt: row.CreatedAt.Time}, nil
}

func (s *messageService) GetUserByTGID(ctx context.Context, chatID uuid.UUID, tgMessageID int64) (domain.UserMsgRecord, error) {
	row, err := s.queries.GetUserMessageByTelegramID(ctx, postgres.GetUserMessageByTelegramIDParams{
		ChatID:            pgtype.UUID{Bytes: chatID, Valid: true},
		TelegramMessageID: pgtype.Int8{Int64: tgMessageID, Valid: true},
	})
	if err != nil {
		return domain.UserMsgRecord{}, err
	}
	return domain.UserMsgRecord{ID: uuid.UUID(row.ID.Bytes), CreatedAt: row.CreatedAt.Time}, nil
}

func (s *messageService) AssistantReplyExistsAfter(ctx context.Context, chatID uuid.UUID, after time.Time) (bool, error) {
	return s.queries.AssistantReplyExistsAfter(ctx, postgres.AssistantReplyExistsAfterParams{
		ChatID:    pgtype.UUID{Bytes: chatID, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: after, Valid: true},
	})
}

func (s *messageService) ListRecent(ctx context.Context, chatID uuid.UUID, limit int32) ([]domain.RecentMessage, error) {
	rows, err := s.queries.ListRecentMessages(ctx, postgres.ListRecentMessagesParams{
		ChatID: pgtype.UUID{Bytes: chatID, Valid: true},
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.RecentMessage, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.RecentMessage{
			Role:      r.Role,
			Content:   r.Content,
			CreatedAt: r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (s *messageService) InsertAssistant(ctx context.Context, args ports.InsertAssistantArgs) error {
	return s.queries.InsertAssistantMessage(ctx, postgres.InsertAssistantMessageParams{
		ChatID:            pgtype.UUID{Bytes: args.ChatID, Valid: true},
		Content:           args.Content,
		TelegramMessageID: pgtype.Int8{Int64: args.TelegramMessageID, Valid: true},
		LlmModel:          pgtype.Text{String: args.Model, Valid: true},
		LlmTokensIn:       pgtype.Int4{Int32: args.TokensIn, Valid: true},
		LlmTokensOut:      pgtype.Int4{Int32: args.TokensOut, Valid: true},
	})
}
