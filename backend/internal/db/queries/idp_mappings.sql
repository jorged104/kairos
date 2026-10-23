-- name: CreateIdpMapping :one
INSERT INTO idp_mappings (issuer, claim, external_value, target_type, target)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIdpMapping :one
SELECT * FROM idp_mappings WHERE id = $1;

-- name: ListIdpMappings :many
SELECT * FROM idp_mappings ORDER BY issuer, claim, external_value;

-- name: ListIdpMappingsByIssuer :many
SELECT * FROM idp_mappings WHERE issuer = $1;

-- name: UpdateIdpMapping :one
UPDATE idp_mappings
SET claim = $2, external_value = $3, target_type = $4, target = $5
WHERE id = $1
RETURNING *;

-- name: DeleteIdpMapping :execrows
DELETE FROM idp_mappings WHERE id = $1;
