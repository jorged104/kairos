-- name: CreateUser :one
INSERT INTO users (name, email, email_normalized, status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserForUpdate :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email_normalized = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET name = $2, email = $3, email_normalized = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetUserStatus :one
UPDATE users SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: LockAdmins :exec
-- Serializes every operation that may remove an active administrator (RF-38, RF-79).
SELECT pg_advisory_xact_lock(hashtext('kairos_admins'));

-- name: CountActiveAdmins :one
SELECT count(*) FROM user_roles r
JOIN users u ON u.id = r.user_id
WHERE r.role = 'admin' AND u.status = 'active';

-- name: GetCredential :one
SELECT * FROM local_credentials WHERE user_id = $1;

-- name: UpsertCredential :exec
INSERT INTO local_credentials (user_id, password_hash)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now();

-- name: GetIdentity :one
SELECT * FROM user_identities WHERE issuer = $1 AND subject = $2;

-- name: CreateIdentity :one
INSERT INTO user_identities (user_id, issuer, subject)
VALUES ($1, $2, $3)
ON CONFLICT (issuer, subject) DO NOTHING
RETURNING *;

-- name: ListUserRoles :many
SELECT role, source FROM user_roles WHERE user_id = $1 ORDER BY role;

-- name: ListRolesForUsers :many
SELECT user_id, role, source FROM user_roles WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[]);

-- name: AddManualRole :exec
-- A manual assignment wins over an IdP one for the same role (RF-32).
INSERT INTO user_roles (user_id, role, source)
VALUES ($1, $2, 'manual')
ON CONFLICT (user_id, role) DO UPDATE SET source = 'manual';

-- name: AddIdpRole :execrows
INSERT INTO user_roles (user_id, role, source)
VALUES ($1, $2, 'idp')
ON CONFLICT (user_id, role) DO NOTHING;

-- name: DeleteRole :execrows
DELETE FROM user_roles WHERE user_id = $1 AND role = $2;

-- name: GetUserRole :one
SELECT role, source FROM user_roles WHERE user_id = $1 AND role = $2;
