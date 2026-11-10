-- Spec 023: what the principal needs to know which projects a user sees.

-- name: DirectTeamIDs :many
SELECT team_id FROM team_members WHERE user_id = $1;

-- name: SeenTeamIDs :many
-- The user's teams, their ancestors and their descendants (spec 023, Conceptos 3).
WITH RECURSIVE up AS (
    SELECT m.team_id AS id FROM team_members m WHERE m.user_id = $1
    UNION
    SELECT t.parent_id FROM teams t JOIN up ON t.id = up.id WHERE t.parent_id IS NOT NULL
), down AS (
    SELECT m.team_id AS id FROM team_members m WHERE m.user_id = $1
    UNION
    SELECT c.id FROM teams c JOIN down ON c.parent_id = down.id
)
SELECT up.id::uuid FROM up UNION SELECT down.id::uuid FROM down;

-- name: AssignedProjectIDs :many
-- Projects where the user has a work item assigned, open or closed (Conceptos 5).
SELECT DISTINCT w.project_id FROM work_item_assignees a JOIN work_items w ON w.id = a.work_item_id WHERE a.user_id = $1;

-- name: VisibilityProjects :many
-- Every project that can be visible, with what the rule reads.
SELECT id, parent_id, team_id, owner_id, is_program FROM projects WHERE NOT is_template AND NOT is_inbox;

-- name: HasAssignedWorkItem :one
-- Whether the user has a work item assigned in the project, read live for
-- the access to one project (spec 023, RF-15, RF-16).
SELECT EXISTS (
    SELECT 1 FROM work_item_assignees a JOIN work_items w ON w.id = a.work_item_id
    WHERE a.user_id = sqlc.arg(user_id)::uuid AND w.project_id = sqlc.arg(project_id)::uuid
);
