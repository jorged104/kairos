-- name: CreateAttribute :one
INSERT INTO attributes (name, name_normalized, type, required, options)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAttribute :one
SELECT * FROM attributes WHERE id = $1;

-- name: ListAttributes :many
SELECT * FROM attributes ORDER BY name;

-- name: UpdateAttribute :one
UPDATE attributes
SET name = $2, name_normalized = $3, type = $4, required = $5, options = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetAttributeRetired :one
UPDATE attributes
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountAttributeValues :one
SELECT count(*) FROM user_attribute_values WHERE attribute_id = $1;

-- name: ListUserAttributeValues :many
SELECT attribute_id, value FROM user_attribute_values WHERE user_id = $1;

-- name: ListAttributeValuesForUsers :many
SELECT user_id, attribute_id, value FROM user_attribute_values
WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[]);

-- name: UpsertUserAttributeValue :exec
INSERT INTO user_attribute_values (user_id, attribute_id, value)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, attribute_id) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: DeleteUserAttributeValue :exec
DELETE FROM user_attribute_values WHERE user_id = $1 AND attribute_id = $2;
