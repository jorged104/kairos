-- name: CreateMilestone :one
INSERT INTO milestones (project_id, phase_id, name, description, description_text, due_date, position)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: GetMilestone :one
SELECT * FROM milestones WHERE id = $1;

-- name: GetMilestoneForUpdate :one
SELECT * FROM milestones WHERE id = $1 FOR UPDATE;

-- name: UpdateMilestone :one
UPDATE milestones
SET phase_id = $3, name = $4, description = $5, description_text = $6, due_date = $7, position = $8,
    version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2 RETURNING *;

-- name: SetMilestoneStatus :one
UPDATE milestones SET status = $3, completed_on = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2 RETURNING *;

-- name: MilestoneInUse :one
SELECT (EXISTS (SELECT 1 FROM milestone_work_items mw WHERE mw.milestone_id = sqlc.arg(id)::uuid)
     OR EXISTS (SELECT 1 FROM objective_milestones om WHERE om.milestone_id = sqlc.arg(id)::uuid))::boolean AS used;

-- name: DeleteMilestone :exec
DELETE FROM milestones WHERE id = $1;

-- name: LinkMilestoneWorkItem :exec
INSERT INTO milestone_work_items (milestone_id, work_item_id, project_id) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: UnlinkMilestoneWorkItem :execrows
DELETE FROM milestone_work_items WHERE milestone_id = $1 AND work_item_id = $2;

-- name: MilestoneProgress :many
-- Progress of the milestones of a project, or of one milestone, over their
-- effective work items: linked ones plus all their descendants, each once
-- (RF-52, RF-55, RF-154). Hours are the items' own, never derived; weight 0
-- adds hours but no progress (RF-159).
WITH RECURSIVE eff AS (
    SELECT mw.milestone_id, mw.work_item_id AS id
    FROM milestone_work_items mw JOIN milestones m ON m.id = mw.milestone_id
    WHERE (sqlc.narg(project_id)::uuid IS NULL OR m.project_id = sqlc.narg(project_id))
      AND (sqlc.narg(milestone_id)::uuid IS NULL OR m.id = sqlc.narg(milestone_id))
    UNION
    SELECT eff.milestone_id, c.id FROM work_items c JOIN eff ON c.parent_id = eff.id
)
SELECT eff.milestone_id,
       count(*)::int AS total,
       (count(*) FILTER (WHERE s.is_closed))::int AS closed,
       coalesce(sum(w.estimated_hours), 0)::float8 AS estimated_hours,
       coalesce(sum(w.remaining_hours), 0)::float8 AS remaining_hours,
       coalesce(sum(w.weight), 0)::bigint AS weight,
       coalesce(sum(w.weight) FILTER (WHERE s.is_closed), 0)::bigint AS weight_closed
FROM eff
JOIN work_items w ON w.id = eff.id
JOIN statuses s ON s.id = w.status_id
GROUP BY eff.milestone_id;

-- name: ListProjectMilestones :many
SELECT m.*, ph.id AS phase_ref_id, d.name AS phase_name
FROM milestones m
LEFT JOIN project_phases ph ON ph.id = m.phase_id
LEFT JOIN phase_definitions d ON d.id = ph.definition_id
WHERE m.project_id = $1
ORDER BY m.position, m.due_date NULLS LAST, m.name;

-- name: MilestoneEffectiveWorkItems :many
-- The effective work items of a milestone, saying which are linked directly (RF-52).
WITH RECURSIVE eff AS (
    SELECT mw.work_item_id AS id, true AS direct FROM milestone_work_items mw WHERE mw.milestone_id = $1
    UNION
    SELECT c.id, false FROM work_items c JOIN eff ON c.parent_id = eff.id
)
SELECT w.id, w.parent_id, w.sequence_number, w.title, w.type_id, w.status_id, w.priority_id,
       w.assignee_id, u.name AS assignee_name, w.weighted_done_ratio AS derived_done_ratio, w.agg_estimated, w.agg_remaining,
       w.depth, w.position, w.due_date, w.weight, w.size_id, bool_or(eff.direct)::boolean AS direct
FROM eff JOIN work_items w ON w.id = eff.id LEFT JOIN users u ON u.id = w.assignee_id
GROUP BY w.id, u.name
ORDER BY w.depth, w.position, w.sequence_number;

-- name: WorkItemEffectiveMilestones :many
-- The milestones of a work item: its own plus those of its ancestors, each
-- once, saying which are direct (RF-53).
WITH RECURSIVE up AS (
    SELECT w.id, w.parent_id, 0 AS level FROM work_items w WHERE w.id = $1
    UNION ALL
    SELECT p.id, p.parent_id, up.level + 1 FROM work_items p JOIN up ON p.id = up.parent_id WHERE up.level < 20
)
SELECT m.id, m.name, m.status, m.due_date, bool_or(up.level = 0)::boolean AS direct
FROM up
JOIN milestone_work_items mw ON mw.work_item_id = up.id
JOIN milestones m ON m.id = mw.milestone_id
GROUP BY m.id
ORDER BY m.due_date NULLS LAST, m.name;
