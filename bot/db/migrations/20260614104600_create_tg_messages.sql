-- +goose Up
-- +goose StatementBegin
CREATE TABLE tg_messages (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id              UUID        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role                 TEXT        NOT NULL,
    content              TEXT        NOT NULL,
    telegram_message_id  BIGINT      NULL,
    attachment_kind      TEXT        NULL,
    attachment_file_id   TEXT        NULL,
    llm_model            TEXT        NULL,
    llm_tokens_in        INTEGER     NULL,
    llm_tokens_out       INTEGER     NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tg_messages_chat_created_idx ON tg_messages (chat_id, created_at);
CREATE UNIQUE INDEX tg_messages_chat_tgmsg_unique
    ON tg_messages (chat_id, telegram_message_id)
    WHERE telegram_message_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tg_messages;
-- +goose StatementEnd
