-- +goose Up
-- +goose StatementBegin
CREATE TABLE contexts (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id       UUID        NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    model_config_id    UUID        NOT NULL REFERENCES model_configs(id) ON DELETE RESTRICT,
    slug               TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    description        TEXT        NOT NULL,
    prompt             TEXT        NOT NULL,
    is_active          BOOLEAN     NOT NULL DEFAULT FALSE,
    is_age_restricted  BOOLEAN     NOT NULL DEFAULT FALSE,
    position           INTEGER     NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (character_id, slug),
    UNIQUE (character_id, name)
);

CREATE INDEX contexts_character_id_idx ON contexts (character_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE contexts;
-- +goose StatementEnd
