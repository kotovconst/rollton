package openrouter

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors. Compose with *APIError via errors.Join, so callers can use
// either errors.Is(err, ErrX) for class-level branching or errors.As(err, &APIError{})
// to read status/code/message/RetryAfter.
var (
	// ErrInvalidAuth: HTTP 401. Operator must rotate the key.
	ErrInvalidAuth = errors.New("openrouter: invalid auth")

	// ErrInsufficientCredits: HTTP 402. Account out of credits;
	// service layer should alert ops + return "service unavailable" to user.
	ErrInsufficientCredits = errors.New("openrouter: insufficient credits")

	// ErrRateLimited: HTTP 429. Caller may back off using APIError.RetryAfter.
	ErrRateLimited = errors.New("openrouter: rate limited")

	// ErrUpstream: any other 4xx/5xx, malformed response, or transport error.
	ErrUpstream = errors.New("openrouter: upstream error")
)

// APIError carries the OpenRouter error envelope and (when known) HTTP status.
// Always joined with a sentinel via errors.Join — never returned alone.
type APIError struct {
	StatusCode int
	Code       string        // OpenRouter's error.code (may be empty)
	Message    string        // OpenRouter's error.message (may be empty)
	RetryAfter time.Duration // populated only when ErrRateLimited
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("openrouter: status=%d code=%q message=%q", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openrouter: status=%d", e.StatusCode)
}
