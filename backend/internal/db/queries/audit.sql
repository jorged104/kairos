-- name: InsertAudit :exec
INSERT INTO audit_log (actor_id, actor_kind, action, target_type, target_id, before, after)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAudit :many
SELECT a.id, a.occurred_at, a.actor_id, a.actor_kind, a.action, a.target_type, a.target_id,
       a.before, a.after, actor.name AS actor_name, target.name AS target_name
FROM audit_log a
LEFT JOIN users actor ON actor.id = a.actor_id
LEFT JOIN users target ON a.target_type = 'user' AND target.id::text = a.target_id
WHERE (sqlc.narg(target_id)::text IS NULL OR a.target_id = sqlc.narg(target_id))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR a.occurred_at >= sqlc.narg(from_time))
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR a.occurred_at < sqlc.narg(to_time))
ORDER BY a.occurred_at DESC, a.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountAudit :one
SELECT count(*) FROM audit_log
WHERE (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id))
  AND (sqlc.narg(actor_id)::uuid IS NULL OR actor_id = sqlc.narg(actor_id))
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR occurred_at >= sqlc.narg(from_time))
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR occurred_at < sqlc.narg(to_time));
