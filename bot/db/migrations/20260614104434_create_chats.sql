-- +goose Up
-- +goose StatementBegin
CREATE TABLE chats (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    character_id  UUID        NOT NULL REFERENCES characters(id) ON DELETE RESTRICT,
    context_id    UUID        NOT NULL REFERENCES contexts(id) ON DELETE RESTRICT,
    status        TEXT        NOT NULL DEFAULT 'active',
    summary       TEXT        NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chats_user_status_updated_idx ON chats (user_id, status, updated_at DESC);
CREATE INDEX chats_character_id_idx       ON chats (character_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE chats;
-- +goose StatementEnd
