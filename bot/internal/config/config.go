// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DB            DBConfig
	HTTP          HTTPConfig
	Log           LogConfig
	TelegramToken string
	OpenRouter    OpenRouterConfig
}

type DBConfig struct {
	URL string
}

type HTTPConfig struct {
	Port           int
	AllowedOrigins []string
}

type LogConfig struct {
	Level  string
	Format string
}

type OpenRouterConfig struct {
	APIKey  string
	AppURL  string
	AppName string
}

// Load reads env vars (and a local `.env` if present in cwd).
// TELEGRAM_TOKEN is per-process: each cmd/bot-X sets it before calling Load
// (or relies on the runtime env). DATABASE_URL is shared across bots.
func Load() (Config, error) {
	_ = godotenv.Load() // ignore: missing .env is fine in prod

	cfg := Config{
		DB: DBConfig{URL: os.Getenv("DATABASE_URL")},
		HTTP: HTTPConfig{
			Port:           getEnvInt("HTTP_PORT", 8080),
			AllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS"),
		},
		Log:           LogConfig{Level: getEnvStr("LOG_LEVEL", "info"), Format: getEnvStr("LOG_FORMAT", "json")},
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		OpenRouter: OpenRouterConfig{
			APIKey:  os.Getenv("OPENROUTER_API_KEY"),
			AppURL:  os.Getenv("OPENROUTER_APP_URL"),
			AppName: os.Getenv("OPENROUTER_APP_NAME"),
		},
	}

	if cfg.DB.URL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	if cfg.TelegramToken == "" {
		return Config{}, errors.New("config: TELEGRAM_TOKEN is required")
	}
	return cfg, nil
}

func getEnvStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func getEnvList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
