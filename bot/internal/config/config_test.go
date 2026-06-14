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

	cfg, err := config.Load()
	require.NoError(t, err)

	require.Equal(t, "postgres://u:p@localhost:5432/db?sslmode=disable", cfg.DB.URL)
	require.Equal(t, 8081, cfg.HTTP.Port)
	require.Equal(t, "debug", cfg.Log.Level)
	require.Equal(t, "text", cfg.Log.Format)
	require.Equal(t, "abc123", cfg.TelegramToken)
	require.Equal(t, []string{"https://app.rollton.com", "https://example.dev"}, cfg.HTTP.AllowedOrigins)
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

func TestLoad_MissingTelegramToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "")
	_, err := config.Load()
	require.Error(t, err)
}
