package test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/core/services"
)

type fakeUserRepo struct {
	stored      domain.User
	getErr      error
	upsertErr   error
	upsertCalls int
	upsertInput domain.TelegramUserInput
}

func (f *fakeUserRepo) GetByTelegramID(_ context.Context, _ int64) (domain.User, error) {
	if f.getErr != nil {
		return domain.User{}, f.getErr
	}
	return f.stored, nil
}

func (f *fakeUserRepo) UpsertFromTelegram(_ context.Context, in domain.TelegramUserInput) (domain.User, error) {
	f.upsertCalls++
	f.upsertInput = in
	if f.upsertErr != nil {
		return domain.User{}, f.upsertErr
	}
	return domain.User{
		ID:           uuid.New(),
		TelegramID:   in.TelegramID,
		Username:     in.Username,
		FirstName:    in.FirstName,
		LastName:     in.LastName,
		LanguageCode: in.LanguageCode,
		IsPremium:    in.IsPremium,
	}, nil
}

func sampleInput() domain.TelegramUserInput {
	return domain.TelegramUserInput{
		TelegramID:   42,
		Username:     "alice",
		FirstName:    "Alice",
		LastName:     "Smith",
		LanguageCode: "en",
		IsPremium:    false,
	}
}

func storedFromInput(in domain.TelegramUserInput) domain.User {
	return domain.User{
		ID:           uuid.New(),
		TelegramID:   in.TelegramID,
		Username:     in.Username,
		FirstName:    in.FirstName,
		LastName:     in.LastName,
		LanguageCode: in.LanguageCode,
		IsPremium:    in.IsPremium,
	}
}

func TestEnsureRegistered_FoundWithMatchingFields_SkipsUpsert(t *testing.T) {
	in := sampleInput()
	repo := &fakeUserRepo{stored: storedFromInput(in)}
	svc := services.NewUserService(repo)

	u, err := svc.EnsureRegistered(context.Background(), in)

	require.NoError(t, err)
	require.Equal(t, int64(42), u.TelegramID)
	require.Equal(t, 0, repo.upsertCalls)
}

func TestEnsureRegistered_FieldDrift_TriggersUpsert(t *testing.T) {
	in := sampleInput()
	stored := storedFromInput(in)
	stored.Username = "old_handle" // drifted
	repo := &fakeUserRepo{stored: stored}
	svc := services.NewUserService(repo)

	u, err := svc.EnsureRegistered(context.Background(), in)

	require.NoError(t, err)
	require.Equal(t, 1, repo.upsertCalls)
	require.Equal(t, "alice", u.Username)
}

func TestEnsureRegistered_NotFound_CallsUpsert(t *testing.T) {
	repo := &fakeUserRepo{getErr: domain.ErrUserNotFound}
	svc := services.NewUserService(repo)

	u, err := svc.EnsureRegistered(context.Background(), sampleInput())

	require.NoError(t, err)
	require.Equal(t, 1, repo.upsertCalls)
	require.Equal(t, int64(42), u.TelegramID)
}

func TestEnsureRegistered_GetErrorOtherThanNotFound_Bubbles(t *testing.T) {
	repoErr := errors.New("connection refused")
	repo := &fakeUserRepo{getErr: repoErr}
	svc := services.NewUserService(repo)

	_, err := svc.EnsureRegistered(context.Background(), sampleInput())

	require.ErrorIs(t, err, repoErr)
	require.Equal(t, 0, repo.upsertCalls)
}
