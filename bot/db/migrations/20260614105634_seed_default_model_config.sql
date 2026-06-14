-- +goose Up
-- +goose StatementBegin
INSERT INTO model_configs (slug, display_name, provider, model, temperature, max_tokens, is_active)
VALUES (
    'default-fast',
    'Default (Claude Haiku 4.5)',
    'openrouter',
    'anthropic/claude-haiku-4.5',
    0.8,
    2048,
    TRUE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM model_configs WHERE slug = 'default-fast';
-- +goose StatementEnd
