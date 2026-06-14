-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN active_chat_id   UUID        NULL REFERENCES chats(id) ON DELETE SET NULL,
    ADD COLUMN is_adult         BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN age_verified_at  TIMESTAMPTZ NULL;

CREATE INDEX users_active_chat_id_idx ON users (active_chat_id) WHERE active_chat_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_active_chat_id_idx;
ALTER TABLE users
    DROP COLUMN IF EXISTS active_chat_id,
    DROP COLUMN IF EXISTS is_adult,
    DROP COLUMN IF EXISTS age_verified_at;
-- +goose StatementEnd
