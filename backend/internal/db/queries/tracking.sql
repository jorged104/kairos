-- Spec 011: seguimiento de varios proyectos.

-- name: TrackingProjects :many
-- The active projects that are not templates, to choose the ones followed.
SELECT p.id, p.identifier, p.name, p.team_id, p.owner_id, u.name AS owner_name
FROM projects p JOIN users u ON u.id = p.owner_id
WHERE p.lifecycle = 'activo' AND NOT p.is_template AND NOT p.is_inbox
ORDER BY p.name, p.identifier;

-- name: TrackingMilestones :many
SELECT m.id, m.project_id, m.name, m.due_date, m.status
FROM milestones m
WHERE m.project_id = ANY (sqlc.arg(ids)::uuid[]) AND m.status <> 'cancelado'
ORDER BY m.due_date NULLS LAST, m.position, m.name;

-- name: TrackingMilestonePairs :many
-- The effective work items of those milestones (spec 004, RF-52).
WITH RECURSIVE eff AS (
    SELECT mw.milestone_id, mw.work_item_id AS id
    FROM milestone_work_items mw JOIN milestones m ON m.id = mw.milestone_id
    WHERE m.project_id = ANY (sqlc.arg(ids)::uuid[]) AND m.status <> 'cancelado'
    UNION
    SELECT eff.milestone_id, c.id FROM work_items c JOIN eff ON c.parent_id = eff.id
)
SELECT milestone_id, id AS work_item_id FROM eff;

-- name: TrackingLeafItems :many
-- The work items without children of those projects (RF-30).
SELECT w.id, w.project_id, p.identifier, w.sequence_number, w.title, par.title AS parent_title,
       w.start_date, w.due_date, w.closed_at, s.is_closed, s.is_default,
       coalesce(w.weighted_done_ratio, 0)::int AS ratio
FROM work_items w
JOIN projects p ON p.id = w.project_id
JOIN statuses s ON s.id = w.status_id
LEFT JOIN work_items par ON par.id = w.parent_id
WHERE w.project_id = ANY (sqlc.arg(ids)::uuid[])
  AND NOT EXISTS (SELECT 1 FROM work_items c WHERE c.parent_id = w.id);
