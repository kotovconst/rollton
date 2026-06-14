-- +goose Up
-- +goose StatementBegin
CREATE TABLE model_configs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          TEXT        NOT NULL UNIQUE,
    display_name  TEXT        NOT NULL,
    provider      TEXT        NOT NULL,
    model         TEXT        NOT NULL,
    temperature   DOUBLE PRECISION NULL,
    top_p         DOUBLE PRECISION NULL,
    max_tokens    INTEGER     NULL,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE model_configs;
-- +goose StatementEnd
