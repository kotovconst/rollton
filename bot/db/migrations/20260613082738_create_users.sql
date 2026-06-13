-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id     BIGINT      NOT NULL UNIQUE,
    username        TEXT,
    first_name      TEXT        NOT NULL,
    last_name       TEXT,
    language_code   TEXT,
    is_premium      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX users_telegram_id_idx ON users (telegram_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
