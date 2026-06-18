-- name: ListActiveCharacters :many
SELECT * FROM characters
WHERE is_active = TRUE
ORDER BY position ASC, created_at ASC;

-- name: GetCharacterByID :one
SELECT * FROM characters WHERE id = $1;
