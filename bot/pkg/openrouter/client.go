// Package openrouter is a minimal HTTP client for the OpenRouter
// chat-completion API. See docs/superpowers/specs/2026-06-15-rollton-openrouter-client-design.md.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// defaultBaseURL is the production OpenRouter API endpoint.
const defaultBaseURL = "https://openrouter.ai/api/v1"

// Client is a configured OpenRouter HTTP client. Use New + options to construct.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	appURL     string // sent as HTTP-Referer if non-empty
	appName    string // sent as X-Title if non-empty
}

// Option mutates a Client during construction.
type Option func(*Client)

// WithHTTPClient swaps the underlying HTTP client. Use in tests + for custom timeouts.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithBaseURL overrides the API base URL. Use in tests against httptest.NewServer.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithApp sets the HTTP-Referer + X-Title headers OpenRouter uses for usage attribution.
func WithApp(url, name string) Option {
	return func(c *Client) {
		c.appURL = url
		c.appName = name
	}
}

// New constructs a Client with the provided API key.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// wire types (private; mirror OpenRouter's JSON shape exactly).

type wireRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
}

type wireMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type wireResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []wireChoice `json:"choices"`
	Usage   wireUsage    `json:"usage"`
	Error   *wireError   `json:"error,omitempty"`
}

type wireChoice struct {
	Message      wireMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error wireError `json:"error"`
}

// Complete sends a single chat-completion request.
func (c *Client) Complete(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := wireRequest{
		Model:       req.Model,
		Messages:    make([]wireMessage, 0, len(req.Messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, wireMessage{Role: m.Role, Content: m.Content})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, fmt.Errorf("marshal: %w", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, fmt.Errorf("build request: %w", err))
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if c.appURL != "" {
		httpReq.Header.Set("HTTP-Referer", c.appURL)
	}
	if c.appName != "" {
		httpReq.Header.Set("X-Title", c.appName)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Context errors pass through (caller uses errors.Is(err, context.Canceled)).
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ChatResponse{}, ctxErr
		}
		return ChatResponse{}, errors.Join(ErrUpstream, err)
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, fmt.Errorf("read body: %w", err))
	}

	if resp.StatusCode >= 400 {
		return ChatResponse{}, mapHTTPError(resp.StatusCode, resp.Header, rawBody)
	}

	var wr wireResponse
	if err := json.Unmarshal(rawBody, &wr); err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, &APIError{StatusCode: resp.StatusCode, Message: "malformed response body"})
	}
	if len(wr.Choices) == 0 {
		return ChatResponse{}, errors.Join(ErrUpstream, &APIError{StatusCode: resp.StatusCode, Message: "no choices in response"})
	}
	return ChatResponse{
		Model:        wr.Model,
		Reply:        wr.Choices[0].Message.Content,
		TokensIn:     wr.Usage.PromptTokens,
		TokensOut:    wr.Usage.CompletionTokens,
		FinishReason: wr.Choices[0].FinishReason,
	}, nil
}

func mapHTTPError(status int, hdr http.Header, body []byte) error {
	apiErr := &APIError{StatusCode: status}
	var env errorEnvelope
	if json.Unmarshal(body, &env) == nil {
		apiErr.Code = env.Error.Code
		apiErr.Message = env.Error.Message
	}

	switch status {
	case http.StatusUnauthorized:
		return errors.Join(ErrInvalidAuth, apiErr)
	case http.StatusPaymentRequired:
		return errors.Join(ErrInsufficientCredits, apiErr)
	case http.StatusTooManyRequests:
		if ra := hdr.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				apiErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return errors.Join(ErrRateLimited, apiErr)
	default:
		return errors.Join(ErrUpstream, apiErr)
	}
}
