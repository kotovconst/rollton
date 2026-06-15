// Package openrouter is a minimal HTTP client for the OpenRouter
// chat-completion API. See docs/superpowers/specs/2026-06-15-rollton-openrouter-client-design.md.
package openrouter

import (
	"net/http"
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

// WithHTTPClient swaps the underlying HTTP client. Use in tests + for custom
// timeouts. Default: http.DefaultClient.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithBaseURL overrides the API base URL. Use in tests against httptest.NewServer.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithApp sets the HTTP-Referer + X-Title headers OpenRouter uses for usage
// attribution. Both required for the headers to be sent; empty disables.
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
