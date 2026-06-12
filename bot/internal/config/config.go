// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DB            DBConfig
	HTTP          HTTPConfig
	Log           LogConfig
	TelegramToken string
}

type DBConfig struct {
	URL string
}

type HTTPConfig struct {
	Port int
}

type LogConfig struct {
	Level  string
	Format string
}

// Load reads env vars (and a local `.env` if present in cwd).
// TELEGRAM_TOKEN is per-process: each cmd/bot-X sets it before calling Load
// (or relies on the runtime env). DATABASE_URL is shared across bots.
func Load() (Config, error) {
	_ = godotenv.Load() // ignore: missing .env is fine in prod

	cfg := Config{
		DB:            DBConfig{URL: os.Getenv("DATABASE_URL")},
		HTTP:          HTTPConfig{Port: getEnvInt("HTTP_PORT", 8080)},
		Log:           LogConfig{Level: getEnvStr("LOG_LEVEL", "info"), Format: getEnvStr("LOG_FORMAT", "json")},
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
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
