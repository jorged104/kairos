-- name: InsertAudit :exec
INSERT INTO audit_log (actor_id, actor_kind, action, target_type, target_id, before, after)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAudit :many
SELECT * FROM audit_log
WHERE (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR occurred_at >= sqlc.narg(from_time))
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR occurred_at < sqlc.narg(to_time))
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountAudit :one
SELECT count(*) FROM audit_log
WHERE (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR occurred_at >= sqlc.narg(from_time))
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR occurred_at < sqlc.narg(to_time));
