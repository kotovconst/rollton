package openrouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// canned successful response — mirrors OpenRouter's actual shape.
const cannedSuccess = `{
  "id": "gen-test",
  "model": "anthropic/claude-haiku-4.5",
  "choices": [{
    "message": {"role": "assistant", "content": "hello world"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 10, "completion_tokens": 5}
}`

func TestComplete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	res, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model:    "anthropic/claude-haiku-4.5",
		Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	require.Equal(t, "anthropic/claude-haiku-4.5", res.Model)
	require.Equal(t, "hello world", res.Reply)
	require.Equal(t, 10, res.TokensIn)
	require.Equal(t, 5, res.TokensOut)
	require.Equal(t, "stop", res.FinishReason)
}

func TestComplete_RequestShape(t *testing.T) {
	temp := 0.7
	topP := 0.9
	maxT := 256

	var (
		gotAuth    string
		gotReferer string
		gotTitle   string
		gotBody    map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("test-key",
		openrouter.WithBaseURL(srv.URL),
		openrouter.WithApp("https://rollton.com", "Rollton"),
	)
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "anthropic/claude-haiku-4.5",
		Messages: []openrouter.Message{
			{Role: openrouter.RoleSystem, Content: "you are alice"},
			{Role: openrouter.RoleUser, Content: "hi"},
		},
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxT,
	})

	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", gotAuth)
	require.Equal(t, "https://rollton.com", gotReferer)
	require.Equal(t, "Rollton", gotTitle)
	require.Equal(t, "anthropic/claude-haiku-4.5", gotBody["model"])
	require.InDelta(t, 0.7, gotBody["temperature"], 0.0001)
	require.InDelta(t, 0.9, gotBody["top_p"], 0.0001)
	require.Equal(t, float64(256), gotBody["max_tokens"])

	msgs, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[0].(map[string]any)["role"])
	require.Equal(t, "you are alice", msgs[0].(map[string]any)["content"])
}

func TestComplete_RequestShape_OmitsNilOptionals(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model:    "anthropic/claude-haiku-4.5",
		Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	_, hasTemp := gotBody["temperature"]
	_, hasTopP := gotBody["top_p"]
	_, hasMaxT := gotBody["max_tokens"]
	require.False(t, hasTemp, "temperature should be omitted when nil")
	require.False(t, hasTopP, "top_p should be omitted when nil")
	require.False(t, hasMaxT, "max_tokens should be omitted when nil")
}

const cannedError401 = `{"error": {"code": "invalid_api_key", "message": "Invalid API key"}}`
const cannedError402 = `{"error": {"code": "insufficient_credits", "message": "Out of credits"}}`
const cannedError429 = `{"error": {"code": "rate_limited", "message": "Slow down"}}`
const cannedError500 = `{"error": {"code": "server_error", "message": "Internal error"}}`

func TestComplete_401_ReturnsErrInvalidAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, cannedError401)
	}))
	defer srv.Close()

	c := openrouter.New("bad", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model:    "x",
		Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})

	require.ErrorIs(t, err, openrouter.ErrInvalidAuth)
	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 401, apiErr.StatusCode)
	require.Equal(t, "invalid_api_key", apiErr.Code)
	require.Contains(t, apiErr.Message, "Invalid API key")
}

func TestComplete_402_ReturnsErrInsufficientCredits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, cannedError402)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrInsufficientCredits)
}

func TestComplete_429_ReturnsErrRateLimited_WithRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, cannedError429)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrRateLimited)
	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 30*time.Second, apiErr.RetryAfter)
}

func TestComplete_500_ReturnsErrUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, cannedError500)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrUpstream)
	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 500, apiErr.StatusCode)
}

type erroringRoundTripper struct{}

func (erroringRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("simulated network error")
}

func TestComplete_NetworkError_ReturnsErrUpstream(t *testing.T) {
	c := openrouter.New("k",
		openrouter.WithBaseURL("http://localhost:1"),
		openrouter.WithHTTPClient(&http.Client{Transport: erroringRoundTripper{}}),
	)
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrUpstream)
}

func TestComplete_ContextCancelled(t *testing.T) {
	// Server sleeps so the client must wait on the cancelled ctx.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Complete(ctx, openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, context.Canceled)
}
