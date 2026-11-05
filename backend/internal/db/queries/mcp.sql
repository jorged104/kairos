-- Conexiones de clientes de IA (spec 016) -----------------------------------------

-- name: SetMCPEnabled :exec
UPDATE system_settings SET mcp_enabled = $1, version = version + 1, updated_at = now() WHERE id;

-- name: CreateMCPConnection :one
INSERT INTO mcp_connections (user_id, name, secret_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CountLiveMCPConnections :one
SELECT count(*) FROM mcp_connections
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: ListMCPConnections :many
-- The live connections of one person, or of everybody with a null user (RF-19, RF-23).
SELECT c.*, u.name AS user_name, u.email AS user_email
FROM mcp_connections c JOIN users u ON u.id = c.user_id
WHERE c.revoked_at IS NULL AND c.expires_at > now()
  AND (sqlc.narg(user_id)::uuid IS NULL OR c.user_id = sqlc.narg(user_id))
ORDER BY c.created_at DESC, c.id;

-- name: GetMCPConnectionForUpdate :one
SELECT * FROM mcp_connections WHERE id = $1 FOR UPDATE;

-- name: RevokeMCPConnection :exec
UPDATE mcp_connections SET revoked_at = now(), revoked_by = $2 WHERE id = $1 AND revoked_at IS NULL;

-- name: MCPConnectionBySecret :one
-- A live connection by the hash of its secret (RF-15, RF-20 to RF-22).
SELECT * FROM mcp_connections
WHERE secret_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: TouchMCPConnection :exec
UPDATE mcp_connections SET last_used_at = now(), last_client = coalesce(sqlc.narg(client)::text, last_client)
WHERE id = sqlc.arg(id)::uuid;
