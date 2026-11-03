-- Spec 008: papeles, capacidad, vistas, límites del kanban y avance diario.

-- Papeles ------------------------------------------------------------------------

-- name: ListUserProjectRoles :many
-- The roles a user has been given on purpose, for the principal (RF-44).
SELECT project_id, role FROM project_members WHERE user_id = $1;

-- name: ListExplicitProjectMembers :many
SELECT user_id, role FROM project_members WHERE project_id = $1;

-- name: GetProjectMember :one
SELECT * FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: UpsertProjectMember :exec
INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)
ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role, updated_at = now();

-- name: DeleteProjectMember :execrows
DELETE FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: ProjectTeamParticipants :many
-- Members of the project's team or any of its subteams (spec 004, participante).
WITH RECURSIVE tree AS (
    SELECT t.id FROM teams t WHERE t.id = $1
    UNION
    SELECT c.id FROM teams c JOIN tree ON c.parent_id = tree.id
)
SELECT DISTINCT m.user_id FROM team_members m JOIN tree ON tree.id = m.team_id;

-- name: TeamPeople :many
-- Name, status, roles and capacity of some users, for the team screen.
SELECT u.id, u.name, u.status, u.weekly_capacity,
       coalesce(array_agg(r.role ORDER BY r.role) FILTER (WHERE r.role IS NOT NULL), '{}')::text[] AS roles
FROM users u LEFT JOIN user_roles r ON r.user_id = u.id
WHERE u.id = ANY (sqlc.arg(ids)::uuid[])
GROUP BY u.id;

-- Carga --------------------------------------------------------------------------

-- name: OpenLoad :many
-- Open weight and open work items assigned to some people: in all projects and
-- in one (RF-48).
SELECT a.user_id,
       coalesce(sum(w.weight), 0)::bigint AS total_weight,
       coalesce(sum(w.weight) FILTER (WHERE w.project_id = sqlc.arg(project_id)::uuid), 0)::bigint AS project_weight,
       (count(*) FILTER (WHERE w.project_id = sqlc.arg(project_id)::uuid))::int AS project_items
FROM work_item_assignees a
JOIN work_items w ON w.id = a.work_item_id
JOIN statuses s ON s.id = w.status_id
JOIN projects p ON p.id = w.project_id
WHERE a.user_id = ANY (sqlc.arg(ids)::uuid[]) AND NOT s.is_closed AND NOT p.is_template
GROUP BY a.user_id;

-- name: UnassignedOpenItems :many
-- Open work items of a project without assignees (RF-51), by due date.
SELECT w.id, w.sequence_number, w.title, w.weight, w.due_date,
       coalesce(array_agg(wl.label_id) FILTER (WHERE wl.label_id IS NOT NULL), '{}')::uuid[] AS label_ids
FROM work_items w
JOIN statuses s ON s.id = w.status_id
LEFT JOIN work_item_labels wl ON wl.work_item_id = w.id
WHERE w.project_id = $1 AND NOT s.is_closed
  AND NOT EXISTS (SELECT 1 FROM work_item_assignees a WHERE a.work_item_id = w.id)
GROUP BY w.id
ORDER BY w.due_date NULLS LAST, w.sequence_number
LIMIT 100;

-- name: PeopleLabels :many
-- The labels of the open work items each person has in a project, to match suggestions (RF-52).
SELECT DISTINCT a.user_id, wl.label_id
FROM work_item_assignees a
JOIN work_items w ON w.id = a.work_item_id
JOIN work_item_labels wl ON wl.work_item_id = w.id
WHERE w.project_id = $1 AND a.user_id = ANY (sqlc.arg(ids)::uuid[]);

-- name: SetUserCapacity :one
UPDATE users SET weekly_capacity = sqlc.narg(capacity), updated_at = now() WHERE id = $1 RETURNING *;

-- Vistas -------------------------------------------------------------------------

-- name: ListVisibleViews :many
-- The shared views of a project and the caller's personal ones (RF-26).
SELECT * FROM project_views
WHERE project_id = $1 AND (owner_id IS NULL OR owner_id = sqlc.arg(user_id)::uuid)
ORDER BY owner_id NULLS FIRST, name;

-- name: GetView :one
SELECT * FROM project_views WHERE id = $1;

-- name: CountViews :one
SELECT count(*)::int FROM project_views
WHERE project_id = $1 AND owner_id IS NOT DISTINCT FROM sqlc.narg(owner_id)::uuid;

-- name: ViewNameTaken :one
-- Whether a name is in use among the views someone sees: their personal ones
-- and the shared ones (RF-29); a shared name is checked against everybody's.
SELECT EXISTS (
    SELECT 1 FROM project_views
    WHERE project_id = $1 AND name_normalized = $2 AND id <> sqlc.arg(except_id)::uuid
      AND (owner_id IS NULL OR sqlc.narg(owner_id)::uuid IS NULL OR owner_id = sqlc.narg(owner_id)::uuid)
);

-- name: CreateView :one
INSERT INTO project_views (project_id, owner_id, name, name_normalized, config, created_by)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: UpdateView :one
UPDATE project_views SET name = $2, name_normalized = $3, config = $4, owner_id = $5, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteView :exec
DELETE FROM project_views WHERE id = $1;

-- Límites del kanban ---------------------------------------------------------------

-- name: ListColumnLimits :many
SELECT status_id, wip_limit FROM project_column_limits WHERE project_id = $1;

-- name: SetColumnLimit :exec
INSERT INTO project_column_limits (project_id, status_id, wip_limit) VALUES ($1, $2, $3)
ON CONFLICT (project_id, status_id) DO UPDATE SET wip_limit = EXCLUDED.wip_limit;

-- name: DeleteColumnLimit :exec
DELETE FROM project_column_limits WHERE project_id = $1 AND status_id = $2;

-- Avance del proyecto ---------------------------------------------------------------

-- name: ProjectProgress :one
-- The progress of a project: its work items' own progress weighted by their
-- weight, closed ones at 100 % (spec 008, Conceptos).
SELECT coalesce(round(sum(w.weight * CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END)::numeric
                      / nullif(sum(w.weight), 0)), 0)::int AS progress
FROM work_items w JOIN statuses s ON s.id = w.status_id
WHERE w.project_id = $1;

-- name: ProjectStatusCounts :many
SELECT w.status_id, count(*)::int AS items
FROM work_items w WHERE w.project_id = $1 GROUP BY w.status_id;

-- name: RecordDailyProgress :exec
-- Today's progress of every active project, again on each run (RF-12).
INSERT INTO project_progress_daily (project_id, day, progress)
SELECT p.id, sqlc.arg(day)::date,
       coalesce(round(sum(w.weight * CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END)::numeric
                      / nullif(sum(w.weight), 0)), 0)::smallint
FROM projects p
LEFT JOIN work_items w ON w.project_id = p.id
LEFT JOIN statuses s ON s.id = w.status_id
WHERE p.lifecycle = 'activo' AND NOT p.is_template
GROUP BY p.id
ON CONFLICT (project_id, day) DO UPDATE SET progress = EXCLUDED.progress, updated_at = now();

-- name: ListDailyProgress :many
SELECT day, progress FROM project_progress_daily
WHERE project_id = $1 AND day > sqlc.arg(since)::date ORDER BY day;

-- name: ProjectOpenItems :many
-- The open work items of a project, to find the ones at risk (RF-15).
SELECT w.id, w.sequence_number, w.title, w.start_date, w.due_date, w.weighted_done_ratio,
       EXISTS (SELECT 1 FROM work_item_dependencies d JOIN work_items b ON b.id = d.from_id
               JOIN statuses bs ON bs.id = b.status_id
               WHERE d.to_id = w.id AND d.type = 'blocks' AND NOT bs.is_closed) AS blocked
FROM work_items w JOIN statuses s ON s.id = w.status_id
WHERE w.project_id = $1 AND NOT s.is_closed;
