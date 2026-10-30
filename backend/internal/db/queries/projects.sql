-- name: LockProjectTree :exec
-- Serializes changes of project parents, so two moves cannot build a cycle.
SELECT pg_advisory_xact_lock(hashtext('project-tree'));

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectForUpdate :one
SELECT * FROM projects WHERE id = $1 FOR UPDATE;

-- name: GetProjectByIdentifier :one
SELECT * FROM projects WHERE identifier = $1;

-- name: CreateProject :one
INSERT INTO projects (parent_id, template_id, team_id, owner_id, methodology_id, identifier, name,
                      description, description_text, is_template, health, health_explanation,
                      start_date, end_date, depth)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects
SET parent_id = $3, team_id = $4, owner_id = $5, name = $6, description = $7, description_text = $8,
    health = $9, health_explanation = $10, start_date = $11, end_date = $12, depth = $13,
    version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING *;

-- name: SetProjectLifecycle :one
UPDATE projects SET lifecycle = $3, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING *;

-- name: SetProjectTemplate :one
UPDATE projects SET is_template = $2, version = version + 1, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ShiftProjectDepths :exec
-- Moves the depth of a subtree (the project included) by delta after a parent change.
WITH RECURSIVE tree AS (
    SELECT p.id FROM projects p WHERE p.id = sqlc.arg(id)::uuid
    UNION ALL
    SELECT c.id FROM projects c JOIN tree ON c.parent_id = tree.id
)
UPDATE projects SET depth = depth + sqlc.arg(delta)::int FROM tree WHERE projects.id = tree.id;

-- name: ProjectSubtreeHeight :one
-- Levels below the project (0 for a leaf).
WITH RECURSIVE tree AS (
    SELECT p.id, 0 AS level FROM projects p WHERE p.id = $1
    UNION ALL
    SELECT c.id, tree.level + 1 FROM projects c JOIN tree ON c.parent_id = tree.id WHERE tree.level < 10
)
SELECT max(level)::int FROM tree;

-- name: ProjectIsDescendant :one
-- Whether candidate is the project or one of its descendants.
WITH RECURSIVE tree AS (
    SELECT p.id FROM projects p WHERE p.id = sqlc.arg(project_id)::uuid
    UNION ALL
    SELECT c.id FROM projects c JOIN tree ON c.parent_id = tree.id
)
SELECT EXISTS (SELECT 1 FROM tree WHERE tree.id = sqlc.arg(candidate)::uuid);

-- name: ProjectSubtree :many
-- Every descendant of a project, in one query (RF-28).
WITH RECURSIVE tree AS (
    SELECT p.id FROM projects p WHERE p.parent_id = $1
    UNION ALL
    SELECT c.id FROM projects c JOIN tree ON c.parent_id = tree.id
)
SELECT p.id, p.parent_id, p.identifier, p.name, p.lifecycle, p.health, p.depth, p.is_template
FROM projects p JOIN tree ON tree.id = p.id
ORDER BY p.depth, p.name;

-- name: ProjectNames :many
-- Names of the related records a project view shows.
SELECT p.id,
       t.name AS team_name,
       u.name AS owner_name,
       m.name AS methodology_name,
       pp.identifier AS parent_identifier,
       pp.name AS parent_name
FROM projects p
JOIN teams t ON t.id = p.team_id
JOIN users u ON u.id = p.owner_id
LEFT JOIN methodologies m ON m.id = p.methodology_id
LEFT JOIN projects pp ON pp.id = p.parent_id
WHERE p.id = ANY (sqlc.arg(ids)::uuid[]);

-- name: ListProjectTypeIDs :many
SELECT type_id FROM project_work_item_types WHERE project_id = $1;

-- name: AddProjectType :exec
INSERT INTO project_work_item_types (project_id, type_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveProjectType :exec
DELETE FROM project_work_item_types WHERE project_id = $1 AND type_id = $2;

-- name: ProjectTypeInUse :one
SELECT EXISTS (SELECT 1 FROM work_items WHERE project_id = $1 AND type_id = $2);

-- name: ProjectTypeEnabled :one
SELECT EXISTS (SELECT 1 FROM project_work_item_types WHERE project_id = $1 AND type_id = $2);

-- Fases --------------------------------------------------------------------

-- name: CreateProjectPhase :one
INSERT INTO project_phases (project_id, definition_id, position, start_date, finish_date)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListProjectPhases :many
SELECT ph.*, d.name, d.has_start_gate, d.has_finish_gate
FROM project_phases ph JOIN phase_definitions d ON d.id = ph.definition_id
WHERE ph.project_id = $1
ORDER BY ph.position, d.name;

-- name: GetProjectPhase :one
SELECT ph.*, d.name, d.has_start_gate, d.has_finish_gate
FROM project_phases ph JOIN phase_definitions d ON d.id = ph.definition_id
WHERE ph.id = $1;

-- name: UpdateProjectPhaseDates :exec
UPDATE project_phases SET start_date = $2, finish_date = $3 WHERE id = $1;

-- name: SetStartGate :exec
UPDATE project_phases SET start_gate_passed_at = sqlc.narg(passed_at), start_gate_passed_by = sqlc.narg(passed_by) WHERE id = $1;

-- name: SetFinishGate :exec
UPDATE project_phases SET finish_gate_passed_at = sqlc.narg(passed_at), finish_gate_passed_by = sqlc.narg(passed_by) WHERE id = $1;

-- name: ListActivePhaseDefinitions :many
SELECT * FROM phase_definitions WHERE methodology_id = $1 AND retired_at IS NULL ORDER BY position, name;

-- Plantillas ----------------------------------------------------------------

-- name: ListTemplates :many
SELECT p.id, p.identifier, p.name, p.team_id, t.name AS team_name, p.methodology_id, m.name AS methodology_name
FROM projects p
JOIN teams t ON t.id = p.team_id
LEFT JOIN methodologies m ON m.id = p.methodology_id
WHERE p.is_template AND p.lifecycle <> 'archivado'
ORDER BY p.name;

-- name: ProjectHasObjectives :one
SELECT EXISTS (SELECT 1 FROM objective_projects WHERE project_id = $1);

-- name: ListProjectWorkItemsForCopy :many
SELECT * FROM work_items WHERE project_id = $1 ORDER BY depth, sequence_number;

-- name: ListProjectDependenciesForCopy :many
SELECT d.* FROM work_item_dependencies d
JOIN work_items a ON a.id = d.from_id
JOIN work_items b ON b.id = d.to_id
WHERE a.project_id = $1 AND b.project_id = $1;

-- name: ListProjectMilestoneLinksForCopy :many
SELECT milestone_id, work_item_id FROM milestone_work_items WHERE project_id = $1;

-- name: ListProjectWorkItemValuesForCopy :many
SELECT v.* FROM work_item_custom_values v JOIN work_items w ON w.id = v.work_item_id WHERE w.project_id = $1;

-- name: CopyWorkItem :one
INSERT INTO work_items (project_id, type_id, status_id, priority_id, parent_id, sequence_number, title,
                        description, description_text, author_id, start_date, due_date,
                        estimated_hours, remaining_hours, position, depth, weight, size_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING id;

-- name: SetWorkItemCounter :exec
UPDATE projects SET wi_counter = $2 WHERE id = $1;

-- name: CopyMilestone :one
INSERT INTO milestones (project_id, phase_id, name, description, description_text, due_date, position)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id;

-- name: CopyDependency :exec
INSERT INTO work_item_dependencies (from_id, to_id, type, lag_days, created_by) VALUES ($1, $2, $3, $4, $5);

-- name: CopyMilestoneLink :exec
INSERT INTO milestone_work_items (milestone_id, work_item_id, project_id) VALUES ($1, $2, $3);

-- name: CopyWorkItemValue :exec
INSERT INTO work_item_custom_values (work_item_id, field_id, value) VALUES ($1, $2, $3);

-- name: RecomputeWorkItemAggregates :exec
SELECT work_item_recompute_project($1);
