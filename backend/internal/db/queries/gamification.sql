-- Spec 014: point ledger, point values, levels and team rankings.

-- name: PointsWorkItem :one
-- What a close is worth depends on the project and the size (RF-9, RF-7).
SELECT w.id, w.size_id, p.is_template
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.id = $1;

-- name: GetPointValue :one
SELECT xp, coins FROM point_values WHERE size_id IS NOT DISTINCT FROM sqlc.narg(size_id)::uuid;

-- name: InsertPointMovement :execrows
-- A repeated event finds its row and writes nothing (RF-12).
INSERT INTO point_ledger (user_id, xp, coins, reason, work_item_id, event_id, reverses_event_id, note, actor_id, created_at)
VALUES (sqlc.arg(user_id), sqlc.arg(xp), sqlc.arg(coins), sqlc.arg(reason), sqlc.narg(work_item_id), sqlc.narg(event_id),
        sqlc.narg(reverses_event_id), sqlc.narg(note), sqlc.narg(actor_id), sqlc.arg(created_at))
ON CONFLICT (event_id, user_id) DO NOTHING;

-- name: LastCloseEvent :one
-- The event of the last close of a work item that paid somebody (RF-14, RF-16).
SELECT event_id::uuid FROM point_ledger
WHERE work_item_id = $1 AND reason = 'close'
ORDER BY seq DESC LIMIT 1;

-- name: CountReversals :one
SELECT count(*) FROM point_ledger WHERE reverses_event_id = $1;

-- name: ListEventMovements :many
SELECT * FROM point_ledger WHERE event_id = $1 ORDER BY seq;

-- name: LockUserPoints :exec
-- Serializes the adjustments of one person, so the 0 floor holds (RF-20).
SELECT pg_advisory_xact_lock(hashtextextended('points:' || sqlc.arg(user_id)::uuid::text, 0));

-- name: UserPointsBalance :one
SELECT COALESCE(sum(xp), 0)::bigint AS xp, COALESCE(sum(coins), 0)::bigint AS coins, COALESCE(max(seq), 0)::bigint AS last_seq
FROM point_ledger WHERE user_id = $1;

-- name: ListUserMovements :many
-- The history, newest first (RF-39).
SELECT l.id, l.seq, l.xp, l.coins, l.reason, l.note, l.created_at, l.work_item_id,
       w.title AS work_item_title, COALESCE(p.identifier || '-' || w.sequence_number, '')::text AS work_item_key
FROM point_ledger l
LEFT JOIN work_items w ON w.id = l.work_item_id
LEFT JOIN projects p ON p.id = w.project_id
WHERE l.user_id = sqlc.arg(user_id)
  AND l.seq > sqlc.arg(after_seq)::bigint
ORDER BY l.seq DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountUserMovements :one
SELECT count(*) FROM point_ledger WHERE user_id = $1;

-- name: ListPointValues :many
-- Active sizes, and retired ones that still have a value (RF-26).
SELECT s.id, s.name, s.position, s.retired_at, COALESCE(v.xp, 0)::int AS xp, COALESCE(v.coins, 0)::int AS coins
FROM work_item_sizes s
LEFT JOIN point_values v ON v.size_id = s.id
WHERE s.retired_at IS NULL OR (v.size_id IS NOT NULL AND (v.xp > 0 OR v.coins > 0))
ORDER BY s.position, s.name;

-- name: GetSize :one
SELECT id, retired_at FROM work_item_sizes WHERE id = $1;

-- name: SetSizePointValue :exec
INSERT INTO point_values (size_id, xp, coins) VALUES ($1, $2, $3)
ON CONFLICT (size_id) DO UPDATE SET xp = EXCLUDED.xp, coins = EXCLUDED.coins, updated_at = now();

-- name: SetNoSizePointValue :exec
UPDATE point_values SET xp = $1, coins = $2, updated_at = now() WHERE size_id IS NULL;

-- name: LockLevels :exec
SELECT pg_advisory_xact_lock(hashtextextended('levels', 0));

-- name: ListLevels :many
SELECT * FROM levels ORDER BY min_xp;

-- name: GetLevel :one
SELECT * FROM levels WHERE id = $1;

-- name: CreateLevel :one
INSERT INTO levels (name, min_xp) VALUES ($1, $2) RETURNING *;

-- name: UpdateLevel :one
UPDATE levels SET name = $2, min_xp = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteLevel :exec
DELETE FROM levels WHERE id = $1;

-- name: TeamRanking :many
-- Active members of the teams with their XP in [from, to) and in total (RF-43
-- to RF-46, RF-49). A null bound is open.
SELECT u.id, u.name,
       COALESCE(sum(l.xp) FILTER (WHERE (sqlc.narg(from_at)::timestamptz IS NULL OR l.created_at >= sqlc.narg(from_at))
                                    AND (sqlc.narg(to_at)::timestamptz IS NULL OR l.created_at < sqlc.narg(to_at))), 0)::bigint AS period_xp,
       COALESCE(sum(l.xp), 0)::bigint AS total_xp
FROM users u
LEFT JOIN point_ledger l ON l.user_id = u.id
WHERE u.status = 'active'
  AND u.id IN (SELECT m.user_id FROM team_members m WHERE m.team_id = ANY(sqlc.arg(team_ids)::uuid[]))
GROUP BY u.id, u.name;
