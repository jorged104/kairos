-- Correo, invitaciones y recuperación de contraseña (spec 019) -------------------

-- name: SetMailSettings :exec
UPDATE system_settings SET
    mail_enabled = $1, smtp_host = $2, smtp_port = $3, smtp_security = $4, smtp_username = $5,
    smtp_password = $6, mail_from_email = $7, mail_from_name = $8,
    version = version + 1, updated_at = now()
WHERE id;

-- name: CreateUserToken :one
INSERT INTO user_tokens (user_id, purpose, token_hash, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: RevokeUserTokens :exec
-- Before a new link of the same purpose (RF-20, RF-28), or of every purpose
-- when the person is deactivated (RF-42).
UPDATE user_tokens SET revoked_at = now()
WHERE user_id = sqlc.arg(user_id)::uuid AND used_at IS NULL AND revoked_at IS NULL
  AND (sqlc.narg(purpose)::text IS NULL OR purpose = sqlc.narg(purpose)::text);

-- name: UserTokenByHash :one
SELECT t.*, u.email, u.name, u.status, u.email_normalized
FROM user_tokens t JOIN users u ON u.id = t.user_id
WHERE t.token_hash = $1;

-- name: UserTokenByHashForUpdate :one
SELECT * FROM user_tokens WHERE token_hash = $1 FOR UPDATE;

-- name: UseUserToken :exec
UPDATE user_tokens SET used_at = now() WHERE id = $1;

-- name: CountRecentResets :one
-- Recoveries asked for a person in the last hour (RF-27).
SELECT count(*) FROM user_tokens
WHERE user_id = $1 AND purpose = 'reset' AND created_by IS NULL AND created_at > sqlc.arg(since)::timestamptz;

-- name: PendingInvitation :one
-- The latest invitation of a person without a password: pending or expired (RF-19).
SELECT t.expires_at FROM user_tokens t
WHERE t.user_id = $1 AND t.purpose = 'invite' AND t.used_at IS NULL AND t.revoked_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM local_credentials c WHERE c.user_id = t.user_id)
ORDER BY t.created_at DESC
LIMIT 1;

-- name: PendingInvitations :many
-- The same for a page of the directory.
SELECT DISTINCT ON (t.user_id) t.user_id, t.expires_at FROM user_tokens t
WHERE t.user_id = ANY (sqlc.arg(ids)::uuid[]) AND t.purpose = 'invite' AND t.used_at IS NULL AND t.revoked_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM local_credentials c WHERE c.user_id = t.user_id)
ORDER BY t.user_id, t.created_at DESC;
