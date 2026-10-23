-- name: CreateObjective :one
INSERT INTO objectives (title, description, description_text, owner_id, scope, scope_team_id, scope_project_id, period_start, period_end)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: GetObjective :one
SELECT * FROM objectives WHERE id = $1;

-- name: GetObjectiveForUpdate :one
SELECT * FROM objectives WHERE id = $1 FOR UPDATE;

-- name: UpdateObjective :one
UPDATE objectives
SET title = $3, description = $4, description_text = $5, owner_id = $6, period_start = $7, period_end = $8,
    version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2 RETURNING *;

-- name: SetObjectiveStatus :one
UPDATE objectives SET status = $3, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2 RETURNING *;

-- name: CreateKeyResult :one
INSERT INTO key_results (objective_id, title, unit, start_value, target_value, current_value, calc_mode, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: GetKeyResult :one
SELECT * FROM key_results WHERE id = $1;

-- name: UpdateKeyResult :one
UPDATE key_results
SET title = $3, unit = $4, start_value = $5, target_value = $6, current_value = $7, calc_mode = $8, position = $9,
    version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2 RETURNING *;

-- name: DeleteKeyResult :exec
DELETE FROM key_results WHERE id = $1;

-- name: CountKeyResults :one
SELECT count(*) FROM key_results WHERE objective_id = $1;

-- name: KeyResultsWithCounts :many
-- Key results of some objectives with what their derived value needs (RF-110).
SELECT kr.*,
       (SELECT count(*) FROM key_result_work_items x WHERE x.key_result_id = kr.id)::int AS work_items_total,
       (SELECT count(*) FROM key_result_work_items x
          JOIN work_items w ON w.id = x.work_item_id JOIN statuses s ON s.id = w.status_id
         WHERE x.key_result_id = kr.id AND s.is_closed)::int AS work_items_done,
       (SELECT count(*) FROM key_result_milestones x WHERE x.key_result_id = kr.id)::int AS milestones_total,
       (SELECT count(*) FROM key_result_milestones x JOIN milestones m ON m.id = x.milestone_id
         WHERE x.key_result_id = kr.id AND m.status = 'cumplido')::int AS milestones_done
FROM key_results kr
WHERE kr.objective_id = ANY (sqlc.arg(objective_ids)::uuid[])
ORDER BY kr.position, kr.created_at;

-- name: LinkObjectiveProject :exec
INSERT INTO objective_projects (objective_id, project_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnlinkObjectiveProject :execrows
DELETE FROM objective_projects WHERE objective_id = $1 AND project_id = $2;

-- name: LinkObjectiveMilestone :exec
INSERT INTO objective_milestones (objective_id, milestone_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnlinkObjectiveMilestone :execrows
DELETE FROM objective_milestones WHERE objective_id = $1 AND milestone_id = $2;

-- name: ObjectiveHasMilestone :one
SELECT EXISTS (SELECT 1 FROM objective_milestones WHERE objective_id = $1 AND milestone_id = $2);

-- name: LinkKeyResultWorkItem :exec
INSERT INTO key_result_work_items (key_result_id, work_item_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnlinkKeyResultWorkItem :execrows
DELETE FROM key_result_work_items WHERE key_result_id = $1 AND work_item_id = $2;

-- name: LinkKeyResultMilestone :exec
INSERT INTO key_result_milestones (key_result_id, objective_id, milestone_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: UnlinkKeyResultMilestone :execrows
DELETE FROM key_result_milestones WHERE key_result_id = $1 AND milestone_id = $2;

-- name: ObjectiveProjects :many
SELECT p.id, p.identifier, p.name, p.lifecycle
FROM objective_projects op JOIN projects p ON p.id = op.project_id
WHERE op.objective_id = $1 ORDER BY p.name;

-- name: ObjectiveMilestones :many
-- Contributing milestones with their project (RF-115).
SELECT m.id, m.name, m.status, m.due_date, m.completed_on, p.id AS project_id, p.identifier, p.name AS project_name
FROM objective_milestones om
JOIN milestones m ON m.id = om.milestone_id
JOIN projects p ON p.id = m.project_id
WHERE om.objective_id = $1
ORDER BY m.due_date NULLS LAST, m.name;

-- name: KeyResultLinks :many
-- Work items and milestones linked to the key results of an objective.
SELECT x.key_result_id, 'work_item'::text AS kind, w.id, (p.identifier || '-' || w.sequence_number)::text AS label, w.title AS name
FROM key_result_work_items x
JOIN key_results kr ON kr.id = x.key_result_id
JOIN work_items w ON w.id = x.work_item_id
JOIN projects p ON p.id = w.project_id
WHERE kr.objective_id = $1
UNION ALL
SELECT x.key_result_id, 'milestone'::text, m.id, p.identifier::text, m.name
FROM key_result_milestones x
JOIN milestones m ON m.id = x.milestone_id
JOIN projects p ON p.id = m.project_id
WHERE x.objective_id = $1;
