-- Spec 012: estado de un proyecto, tablero y vistas de la lista de proyectos.

-- name: SetProjectStatus :one
-- The status and, when its category crosses, the lifecycle (RF-17 to RF-20).
UPDATE projects SET status_id = $3, lifecycle = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING *;

-- name: SetProjectStatusID :exec
-- The status that follows a lifecycle change (RF-21, RF-22); the version was
-- already raised by that change.
UPDATE projects SET status_id = $2 WHERE id = $1;

-- name: BoardProjects :many
-- The projects of the three modes (RF-29); a null status reads as the
-- initial one.
SELECT p.id, p.identifier, p.name, p.parent_id, p.is_program, p.lifecycle, p.health,
       coalesce(p.status_id, (SELECT ps.id FROM project_statuses ps WHERE ps.is_initial))::uuid AS status_id,
       p.team_id, t.name AS team_name, p.owner_id, u.name AS owner_name,
       p.start_date, p.end_date, p.version
FROM projects p
JOIN teams t ON t.id = p.team_id
JOIN users u ON u.id = p.owner_id
WHERE NOT p.is_template AND p.lifecycle <> 'archivado'
ORDER BY p.name, p.identifier;

-- name: BoardBaselineDates :many
-- The dates of the work items in the last baseline of each project (RF-49).
SELECT pb.project_id, bi.start_date, bi.due_date
FROM project_baselines pb
JOIN project_baseline_items bi ON bi.baseline_id = pb.id
WHERE pb.version = (SELECT max(x.version) FROM project_baselines x WHERE x.project_id = pb.project_id);

-- name: BoardMilestones :many
SELECT m.id, m.project_id, m.name, m.due_date, m.status
FROM milestones m
JOIN projects p ON p.id = m.project_id AND NOT p.is_template AND p.lifecycle <> 'archivado'
WHERE m.status <> 'cancelado' AND m.due_date IS NOT NULL
ORDER BY m.due_date, m.position, m.name;

-- Vistas ------------------------------------------------------------------------

-- name: ListVisibleListViews :many
-- The shared views and the caller's personal ones (RF-72).
SELECT * FROM project_list_views
WHERE owner_id IS NULL OR owner_id = sqlc.arg(user_id)::uuid
ORDER BY owner_id NULLS FIRST, name;

-- name: GetListView :one
SELECT * FROM project_list_views WHERE id = $1;

-- name: CountListViews :one
SELECT count(*)::int FROM project_list_views WHERE owner_id IS NOT DISTINCT FROM sqlc.narg(owner_id)::uuid;

-- name: ListViewNameTaken :one
-- Whether a name is in use among the views someone sees (RF-76); a shared
-- name is checked against everybody's.
SELECT EXISTS (
    SELECT 1 FROM project_list_views
    WHERE name_normalized = $1 AND id <> sqlc.arg(except_id)::uuid
      AND (owner_id IS NULL OR sqlc.narg(owner_id)::uuid IS NULL OR owner_id = sqlc.narg(owner_id)::uuid)
);

-- name: CreateListView :one
INSERT INTO project_list_views (owner_id, name, name_normalized, config, created_by)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: UpdateListView :one
UPDATE project_list_views SET name = $2, name_normalized = $3, config = $4, owner_id = $5, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteListView :exec
DELETE FROM project_list_views WHERE id = $1;
