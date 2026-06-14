-- +goose Up
-- +goose StatementBegin
CREATE TABLE characters (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT        NOT NULL UNIQUE,
    name         TEXT        NOT NULL UNIQUE,
    blurb        TEXT        NOT NULL,
    avatar_url   TEXT        NULL,
    base_prompt  TEXT        NOT NULL,
    bot_username TEXT        NOT NULL,
    is_active    BOOLEAN     NOT NULL DEFAULT FALSE,
    position     INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE characters;
-- +goose StatementEnd
