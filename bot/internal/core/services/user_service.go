// Package services contains cross-bot use cases.
package services

import (
	"context"
	"errors"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/ports"
)

// UserService coordinates user registration. Hot path is a single read.
type UserService struct {
	repo ports.UserRepository
}

func NewUserService(repo ports.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// EnsureRegistered returns the stored user if one exists and its Telegram-sourced
// fields match the incoming update; otherwise it upserts.
func (s *UserService) EnsureRegistered(ctx context.Context, input domain.TelegramUserInput) (domain.User, error) {
	u, err := s.repo.GetByTelegramID(ctx, input.TelegramID)
	switch {
	case err == nil && telegramFieldsMatch(u, input):
		return u, nil
	case err == nil || errors.Is(err, domain.ErrUserNotFound):
		return s.repo.UpsertFromTelegram(ctx, input)
	default:
		return domain.User{}, err
	}
}

func telegramFieldsMatch(u domain.User, in domain.TelegramUserInput) bool {
	return u.Username == in.Username &&
		u.FirstName == in.FirstName &&
		u.LastName == in.LastName &&
		u.LanguageCode == in.LanguageCode &&
		u.IsPremium == in.IsPremium
}
