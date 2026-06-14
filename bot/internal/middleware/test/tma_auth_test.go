package test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	initdata "github.com/telegram-mini-apps/init-data-golang"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/middleware"
)

const testBotToken = "1234567890:TESTtoken"

var errSvcDown = errors.New("svc down")

// validInitData builds a properly-signed initData query string for tests.
// initdata.Sign returns the hex hash; we assemble the full
// "query_id=...&user=...&auth_date=...&hash=..." string ourselves and URL-encode it.
func validInitData(t *testing.T, token string, tgID int64, authDate time.Time) string {
	t.Helper()
	userJSON := fmt.Sprintf(`{"id":%d,"first_name":"Alice","username":"alice","language_code":"en","is_premium":false}`, tgID)
	payload := map[string]string{
		"query_id": "AAH123",
		"user":     userJSON,
	}
	hash := initdata.Sign(payload, token, authDate)

	v := url.Values{}
	for k, val := range payload {
		v.Set(k, val)
	}
	v.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	v.Set("hash", hash)
	return v.Encode()
}

func nextOK(captured **domain.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := middleware.UserFromContext(r.Context())
		*captured = u
		w.WriteHeader(http.StatusOK)
	})
}

func TestTmaAuth_Valid_PutsUserInCtx(t *testing.T) {
	stored := domain.User{ID: uuid.New(), TelegramID: 42, FirstName: "Alice"}
	svc := &fakeUserSvc{user: stored}
	var captured *domain.User
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(nextOK(&captured))

	raw := validInitData(t, testBotToken, 42, time.Now())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.called)
	require.NotNil(t, captured)
	require.Equal(t, stored.ID, captured.ID)
}

func TestTmaAuth_MissingHeader_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "UNAUTHORIZED")
	require.False(t, svc.called)
}

func TestTmaAuth_WrongPrefix_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer xyz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, svc.called)
}

func TestTmaAuth_BadHMAC_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	raw := validInitData(t, "9876543210:OTHERtoken", 42, time.Now())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, svc.called)
}

func TestTmaAuth_StaleAuthDate_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 1*time.Hour)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	raw := validInitData(t, testBotToken, 42, time.Now().Add(-2*time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, svc.called)
}

func TestTmaAuth_ServiceError_Returns500(t *testing.T) {
	svc := &fakeUserSvc{err: errSvcDown}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	raw := validInitData(t, testBotToken, 42, time.Now())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.True(t, svc.called)
}
