-- name: LockDomainEvents :exec
-- Serializes the writers so event ids commit in order (plan 004, outbox).
SELECT pg_advisory_xact_lock(hashtext('domain_events'));

-- name: InsertDomainEvent :one
INSERT INTO domain_events (type, actor_id, project_id, subject_type, subject_id, payload)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDomainEventsAfter :many
SELECT * FROM domain_events WHERE id > $1 ORDER BY id LIMIT $2;

-- name: EnsureEventConsumer :one
INSERT INTO event_consumers (name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING last_event_id;

-- name: AdvanceEventConsumer :exec
UPDATE event_consumers SET last_event_id = $2, updated_at = now()
WHERE name = $1 AND last_event_id < $2;

-- name: ListDomainEvents :many
SELECT * FROM domain_events
WHERE (sqlc.narg(type)::text IS NULL OR type = sqlc.narg(type))
  AND (sqlc.narg(subject_id)::uuid IS NULL OR subject_id = sqlc.narg(subject_id))
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: CountDomainEvents :one
SELECT count(*) FROM domain_events
WHERE (sqlc.narg(type)::text IS NULL OR type = sqlc.narg(type))
  AND (sqlc.narg(subject_id)::uuid IS NULL OR subject_id = sqlc.narg(subject_id));
