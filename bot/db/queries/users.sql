-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;

-- name: UpsertUserFromTelegram :one
INSERT INTO users (telegram_id, username, first_name, last_name, language_code, is_premium)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (telegram_id) DO UPDATE SET
    username      = EXCLUDED.username,
    first_name    = EXCLUDED.first_name,
    last_name     = EXCLUDED.last_name,
    language_code = EXCLUDED.language_code,
    is_premium    = EXCLUDED.is_premium,
    updated_at    = NOW()
RETURNING *;
