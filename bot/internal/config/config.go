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
	DB               DBConfig
	HTTP             HTTPConfig
	Log              LogConfig
	TelegramToken    string
	OpenRouter       OpenRouterConfig
	LLMHistoryWindow int32
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
//
// DATABASE_URL is required for every binary that calls Load. TELEGRAM_TOKEN is
// only required by binaries that build a tgbot.Bot directly from it
// (rolltonchatbot, admin); those will error from tgbot.New("") if missing.
// cmd/characterbots reads tokens per-character from BOT_TOKEN_<UPPER(slug)>
// and tolerates an empty TELEGRAM_TOKEN.
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
		LLMHistoryWindow: getEnvInt32("LLM_HISTORY_WINDOW", 20),
	}

	if cfg.DB.URL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
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

func getEnvInt32(key string, def int32) int32 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return int32(n)
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
