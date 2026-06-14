-- +goose Up
-- +goose StatementBegin
-- character_id is fully derivable from chats.context_id → contexts.character_id.
-- Keeping it was denormalization without a consistency guard. Removing.
DROP INDEX IF EXISTS chats_character_id_idx;
ALTER TABLE chats DROP COLUMN IF EXISTS character_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE chats
    ADD COLUMN character_id UUID NULL REFERENCES characters(id) ON DELETE RESTRICT;
UPDATE chats c SET character_id = (SELECT character_id FROM contexts WHERE id = c.context_id);
ALTER TABLE chats ALTER COLUMN character_id SET NOT NULL;
CREATE INDEX chats_character_id_idx ON chats (character_id);
-- +goose StatementEnd
