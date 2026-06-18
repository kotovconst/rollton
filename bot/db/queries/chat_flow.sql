-- name: GetMostRecentChatJoinedForUserCharacter :one
SELECT
    c.id            AS chat_id,
    c.status        AS chat_status,
    c.summary       AS chat_summary,
    c.updated_at    AS chat_updated_at,
    ctx.id          AS context_id,
    ctx.slug        AS context_slug,
    ctx.name        AS context_name,
    ctx.prompt      AS context_prompt,
    ctx.is_age_restricted,
    ch.id           AS character_id,
    ch.slug         AS character_slug,
    ch.name         AS character_name,
    ch.base_prompt  AS character_base_prompt,
    mc.id           AS model_config_id,
    mc.slug         AS model_slug,
    mc.model        AS model_name,
    mc.temperature,
    mc.top_p,
    mc.max_tokens
FROM chats c
JOIN contexts ctx ON ctx.id = c.context_id
JOIN characters ch ON ch.id = ctx.character_id
JOIN model_configs mc ON mc.id = ctx.model_config_id
WHERE c.user_id = $1 AND ch.id = $2
ORDER BY c.updated_at DESC
LIMIT 1;

-- name: GetDefaultContextWithModelForCharacter :one
SELECT
    ctx.id          AS context_id,
    ctx.slug        AS context_slug,
    ctx.name        AS context_name,
    ctx.prompt      AS context_prompt,
    ctx.is_age_restricted,
    mc.id           AS model_config_id,
    mc.slug         AS model_slug,
    mc.model        AS model_name,
    mc.temperature,
    mc.top_p,
    mc.max_tokens
FROM contexts ctx
JOIN model_configs mc ON mc.id = ctx.model_config_id
WHERE ctx.character_id = $1 AND ctx.is_active = TRUE
ORDER BY ctx.position ASC, ctx.created_at ASC
LIMIT 1;

-- name: InsertChat :one
INSERT INTO chats (user_id, context_id)
VALUES ($1, $2)
RETURNING id, status, summary, updated_at;

-- name: InsertUserMessageIdempotent :one
INSERT INTO tg_messages (chat_id, role, content, telegram_message_id)
VALUES ($1, 'user', $2, $3)
ON CONFLICT (chat_id, telegram_message_id)
    WHERE telegram_message_id IS NOT NULL
DO NOTHING
RETURNING id, created_at;

-- name: GetUserMessageByTelegramID :one
SELECT id, created_at
FROM tg_messages
WHERE chat_id = $1 AND telegram_message_id = $2 AND role = 'user';

-- name: AssistantReplyExistsAfter :one
SELECT EXISTS(
    SELECT 1 FROM tg_messages
    WHERE chat_id = $1 AND role = 'assistant' AND created_at > $2
) AS reply_exists;

-- name: ListRecentMessages :many
SELECT role, content, created_at
FROM tg_messages
WHERE chat_id = $1
  AND role IN ('user', 'assistant')
ORDER BY created_at DESC
LIMIT $2;

-- name: InsertAssistantMessage :exec
INSERT INTO tg_messages
    (chat_id, role, content, telegram_message_id, llm_model, llm_tokens_in, llm_tokens_out)
VALUES ($1, 'assistant', $2, $3, $4, $5, $6);

-- name: TouchChatUpdatedAt :exec
UPDATE chats SET updated_at = NOW() WHERE id = $1;
