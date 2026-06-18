package config_test

import (
	"testing"

	"github.com/kotovconst/rollton/bot/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("HTTP_PORT", "8081")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("TELEGRAM_TOKEN", "abc123")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.rollton.com,https://example.dev")
	t.Setenv("OPENROUTER_API_KEY", "or-test-key")
	t.Setenv("OPENROUTER_APP_URL", "https://rollton.com")
	t.Setenv("OPENROUTER_APP_NAME", "Rollton")
	t.Setenv("LLM_HISTORY_WINDOW", "25")

	cfg, err := config.Load()
	require.NoError(t, err)

	require.Equal(t, "postgres://u:p@localhost:5432/db?sslmode=disable", cfg.DB.URL)
	require.Equal(t, 8081, cfg.HTTP.Port)
	require.Equal(t, "debug", cfg.Log.Level)
	require.Equal(t, "text", cfg.Log.Format)
	require.Equal(t, "abc123", cfg.TelegramToken)
	require.Equal(t, []string{"https://app.rollton.com", "https://example.dev"}, cfg.HTTP.AllowedOrigins)
	require.Equal(t, "or-test-key", cfg.OpenRouter.APIKey)
	require.Equal(t, "https://rollton.com", cfg.OpenRouter.AppURL)
	require.Equal(t, "Rollton", cfg.OpenRouter.AppName)
	require.Equal(t, int32(25), cfg.LLMHistoryWindow)
}

func TestLoad_AllowedOrigins_EmptyByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.HTTP.AllowedOrigins)
}

func TestLoad_AllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.com , https://b.com ")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, []string{"https://a.com", "https://b.com"}, cfg.HTTP.AllowedOrigins)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_EmptyTelegramToken_OK(t *testing.T) {
	// TELEGRAM_TOKEN is optional at config time; cmd/characterbots reads tokens
	// per-character from BOT_TOKEN_<UPPER(slug)> and does not need this var.
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "")
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.TelegramToken)
}

func TestLoad_OpenRouter_EmptyByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_APP_URL", "")
	t.Setenv("OPENROUTER_APP_NAME", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.OpenRouter.APIKey)
	require.Empty(t, cfg.OpenRouter.AppURL)
	require.Empty(t, cfg.OpenRouter.AppName)
}

func TestLoad_LLMHistoryWindow_DefaultsTo20(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("LLM_HISTORY_WINDOW", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, int32(20), cfg.LLMHistoryWindow)
}

func TestLoad_LLMHistoryWindow_NegativeFallsBackToDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("LLM_HISTORY_WINDOW", "-5")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, int32(20), cfg.LLMHistoryWindow)
}
