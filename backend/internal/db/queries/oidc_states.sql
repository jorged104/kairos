-- name: InsertOIDCState :exec
INSERT INTO oidc_login_states (state, nonce, code_verifier, return_to) VALUES ($1, $2, $3, $4);

-- name: ConsumeOIDCState :one
-- Single use: the row disappears as it is read (RF-17).
DELETE FROM oidc_login_states WHERE state = $1 RETURNING *;

-- name: PurgeOIDCStates :exec
DELETE FROM oidc_login_states WHERE created_at < sqlc.arg(before)::timestamptz;
