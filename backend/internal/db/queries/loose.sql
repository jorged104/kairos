-- Spec 022: tareas sin proyecto.

-- name: GetInbox :one
SELECT * FROM projects WHERE is_inbox;

-- name: LockInboxCreation :exec
-- Two first tasks at once create a single inbox.
SELECT pg_advisory_xact_lock(hashtext('kairos.inbox'));

-- name: CreateInbox :one
INSERT INTO projects (team_id, owner_id, identifier, name, is_inbox, status_id)
VALUES ($1, $2, $3, $4, true, (SELECT id FROM project_statuses WHERE is_initial))
RETURNING *;

-- name: ProjectIdentifierTaken :one
SELECT EXISTS (SELECT 1 FROM projects WHERE identifier = $1);

-- name: InboxTeam :one
-- The team of the inbox: the creator's first one, else the oldest active one,
-- else any.
SELECT t.id FROM teams t
ORDER BY (t.id IN (SELECT m.team_id FROM team_members m WHERE m.user_id = sqlc.arg(user_id)::uuid)) DESC,
         (t.archived_at IS NULL) DESC, t.created_at, t.id
LIMIT 1;

-- name: LooseTaskType :one
-- The type of the work items without a project: «Tarea» while active, else
-- the first active one.
SELECT * FROM work_item_types
WHERE retired_at IS NULL
ORDER BY (name_normalized = 'tarea') DESC, position, name
LIMIT 1;

-- name: SetWorkItemTeam :exec
UPDATE work_items SET team_id = $2, team_mode = $3 WHERE id = $1;

-- name: TeamMembersForTasks :many
-- The members of a team and its subteams who can take work: active and not
-- only viewers (RF-8, RF-9).
WITH RECURSIVE sub AS (
    SELECT t.id FROM teams t WHERE t.id = $1
    UNION
    SELECT c.id FROM teams c JOIN sub ON c.parent_id = sub.id
)
SELECT DISTINCT u.id, u.name
FROM team_members m
JOIN sub ON sub.id = m.team_id
JOIN users u ON u.id = m.user_id
WHERE u.status = 'active'
  AND EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role <> 'viewer')
ORDER BY u.name, u.id;

-- name: OpenEachGroupsForMember :many
-- The open «each person» team tasks of a team or any of its ancestors that
-- the user has no task of yet (RF-25).
WITH RECURSIVE up AS (
    SELECT t.id, t.parent_id FROM teams t WHERE t.id = sqlc.arg(team_id)::uuid
    UNION
    SELECT p.id, p.parent_id FROM teams p JOIN up ON p.id = up.parent_id
)
SELECT w.*
FROM work_items w
JOIN statuses s ON s.id = w.status_id
WHERE w.team_mode = 'each' AND w.parent_id IS NULL AND NOT s.is_closed
  AND w.team_id IN (SELECT id FROM up)
  AND NOT EXISTS (SELECT 1 FROM work_items c JOIN work_item_assignees a ON a.work_item_id = c.id
                  WHERE c.parent_id = w.id AND a.user_id = sqlc.arg(user_id)::uuid)
ORDER BY w.created_at;

-- name: ClaimableTasks :many
-- The open «one person takes it» tasks nobody has, of some teams (RF-18).
SELECT w.id, w.sequence_number, w.title, w.due_date, w.weight, w.created_at, w.author_id, u.name AS author_name,
       w.team_id, t.name AS team_name
FROM work_items w
JOIN statuses s ON s.id = w.status_id
JOIN teams t ON t.id = w.team_id
JOIN users u ON u.id = w.author_id
WHERE w.team_mode = 'claim' AND NOT s.is_closed
  AND w.team_id = ANY (sqlc.arg(team_ids)::uuid[])
  AND NOT EXISTS (SELECT 1 FROM work_item_assignees a WHERE a.work_item_id = w.id)
ORDER BY w.due_date NULLS LAST, w.created_at;

-- name: TeamTasks :many
-- The team tasks of some teams for Mi equipo: «one person takes it» ones and
-- the groups of «each person» ones, open or closed in the last 14 days
-- (RF-33 to RF-35).
SELECT w.id, w.sequence_number, w.title, w.due_date, w.created_at, w.team_mode, w.team_id, t.name AS team_name,
       s.is_closed, w.closed_at
FROM work_items w
JOIN statuses s ON s.id = w.status_id
JOIN teams t ON t.id = w.team_id
WHERE w.team_id = ANY (sqlc.arg(team_ids)::uuid[]) AND w.parent_id IS NULL
  AND (NOT s.is_closed OR w.closed_at >= now() - interval '14 days')
ORDER BY s.is_closed, w.due_date NULLS LAST, w.created_at;

-- name: TaskPeople :many
-- Who has each of some work items, or each of their children: the assignees
-- with whether it is closed (RF-34, RF-35).
SELECT coalesce(w.parent_id, w.id)::uuid AS task_id, a.user_id, u.name, s.is_closed
FROM work_items w
JOIN statuses s ON s.id = w.status_id
JOIN work_item_assignees a ON a.work_item_id = w.id
JOIN users u ON u.id = a.user_id
WHERE w.id = ANY (sqlc.arg(ids)::uuid[]) OR w.parent_id = ANY (sqlc.arg(ids)::uuid[])
ORDER BY u.name, u.id;

-- name: GroupProgress :many
-- Closed and total children of some «each person» groups (RF-24).
SELECT c.parent_id::uuid AS group_id, count(*) FILTER (WHERE s.is_closed)::int AS done, count(*)::int AS total
FROM work_items c
JOIN statuses s ON s.id = c.status_id
WHERE c.parent_id = ANY (sqlc.arg(ids)::uuid[])
GROUP BY c.parent_id;

-- name: FirstClosedStatus :one
SELECT * FROM statuses WHERE is_closed AND retired_at IS NULL ORDER BY position, name LIMIT 1;

-- name: SetWorkItemStatusDirect :exec
-- The «each person» group follows its children (RF-27).
UPDATE work_items
SET status_id = $2, closed_at = $3, done_ratio = CASE WHEN $3::timestamptz IS NULL THEN 0 ELSE 100 END,
    version = version + 1, updated_at = now()
WHERE id = $1;

-- name: DeleteOpenChildren :many
-- The open children of an «each person» group, deleted with it (RF-28).
DELETE FROM work_items c
USING statuses s
WHERE c.parent_id = $1 AND s.id = c.status_id AND NOT s.is_closed
RETURNING c.id;

-- name: DetachClosedChildren :exec
UPDATE work_items SET parent_id = NULL, depth = 0, team_id = NULL, team_mode = NULL WHERE parent_id = $1;

-- name: MoveWorkItemToProject :one
-- A work item without a project goes to one with a new number (RF-38).
UPDATE work_items
SET project_id = $2, sequence_number = $3, team_id = NULL, team_mode = NULL,
    version = version + 1, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MoveWorkItemFiles :exec
-- The files of the work item and of its comments follow it (RF-38).
UPDATE page_files SET project_id = $2
WHERE (owner_kind = 'work_item' AND owner_id = $1)
   OR (owner_kind = 'work_item_comment' AND owner_id IN (SELECT c.id FROM work_item_comments c WHERE c.work_item_id = $1));

-- name: SetWorkItemAssigneeOnly :one
-- Taking or giving back a task changes only who has it (RF-19, RF-22).
UPDATE work_items SET assignee_id = $2, version = version + 1, updated_at = now()
WHERE id = $1
RETURNING *;
