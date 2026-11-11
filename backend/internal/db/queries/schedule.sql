-- Spec 009: plan, línea base y semáforo.

-- name: GetScheduleSettings :one
SELECT * FROM schedule_settings WHERE id;

-- name: GetScheduleSettingsForUpdate :one
SELECT * FROM schedule_settings WHERE id FOR UPDATE;

-- name: UpdateScheduleSettings :one
UPDATE schedule_settings SET
    upper = $1, risk = $2, late = $3,
    name_ahead = $4, name_on_track = $5, name_at_risk = $6, name_late = $7,
    color_ahead = $8, color_on_track = $9, color_at_risk = $10, color_late = $11,
    version = version + 1, updated_at = now()
WHERE id
RETURNING *;

-- name: ProjectPlanItems :many
-- The work items of a project with their own ratio (closed = 100), current
-- dates and the dates of the latest baseline, if any (RF-9, RF-10).
SELECT w.id, w.parent_id, w.weight, s.is_closed,
       (CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END)::int AS ratio,
       w.start_date, w.due_date,
       (bi.work_item_id IS NOT NULL)::bool AS in_baseline,
       bi.start_date AS base_start, bi.due_date AS base_due
FROM work_items w
JOIN statuses s ON s.id = w.status_id
LEFT JOIN project_baseline_items bi ON bi.work_item_id = w.id AND bi.baseline_id = (
    SELECT b.id FROM project_baselines b WHERE b.project_id = w.project_id ORDER BY b.version DESC LIMIT 1)
WHERE w.project_id = $1;

-- name: ProjectMilestoneItemPairs :many
-- The effective work items of each milestone of a project (spec 004, RF-52).
WITH RECURSIVE eff AS (
    SELECT mw.milestone_id, mw.work_item_id AS id
    FROM milestone_work_items mw JOIN milestones m ON m.id = mw.milestone_id
    WHERE m.project_id = $1
    UNION
    SELECT eff.milestone_id, c.id FROM work_items c JOIN eff ON c.parent_id = eff.id
)
SELECT milestone_id, id AS work_item_id FROM eff;

-- name: LatestBaselineVersion :one
SELECT coalesce(max(version), 0)::int FROM project_baselines WHERE project_id = $1;

-- name: CreateBaseline :one
INSERT INTO project_baselines (project_id, version, reason, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CopyBaselineItems :execrows
-- The current dates of every work item of the project (RF-1), with its key
-- and title so it outlives its deletion (spec 024, RF-24).
INSERT INTO project_baseline_items (baseline_id, work_item_id, start_date, due_date, item_key, title)
SELECT sqlc.arg(baseline_id)::uuid, w.id, w.start_date, w.due_date, p.identifier || '-' || w.sequence_number, w.title
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.project_id = sqlc.arg(project_id)::uuid;

-- name: ListBaselines :many
SELECT b.id, b.version, b.reason, b.created_at, u.id AS author_id, u.name AS author_name
FROM project_baselines b JOIN users u ON u.id = b.created_by
WHERE b.project_id = $1
ORDER BY b.version DESC;

-- name: GetBaselineByVersion :one
SELECT b.id, b.version, b.reason, b.created_at, u.id AS author_id, u.name AS author_name
FROM project_baselines b JOIN users u ON u.id = b.created_by
WHERE b.project_id = $1 AND b.version = $2;

-- name: BaselineCompareItems :many
-- The work items of a baseline as they are now (spec 024): deleted ones only
-- when the baseline kept their key and title (RF-25), and where each one is.
SELECT bi.work_item_id AS id, bi.start_date, bi.due_date, bi.item_key, bi.title AS saved_title,
       w.title, w.project_id, w.sequence_number, p.identifier, p.team_id, p.owner_id,
       coalesce(p.is_inbox, false)::bool AS is_inbox
FROM project_baseline_items bi
LEFT JOIN work_items w ON w.id = bi.work_item_id
LEFT JOIN projects p ON p.id = w.project_id
WHERE bi.baseline_id = $1 AND (w.id IS NOT NULL OR bi.title IS NOT NULL);

-- name: CurrentCompareItems :many
-- The work items of a project with their current dates (spec 024, «Hoy»).
SELECT w.id, w.start_date, w.due_date, w.title, w.sequence_number, p.identifier
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.project_id = $1;
