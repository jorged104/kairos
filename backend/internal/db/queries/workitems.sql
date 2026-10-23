-- name: LockWorkItemTree :exec
-- Serializes structural changes inside a project (parents and dependencies).
SELECT pg_advisory_xact_lock(hashtext('wi-tree:' || sqlc.arg(project_id)::uuid::text));

-- name: LockDependencies :exec
SELECT pg_advisory_xact_lock(hashtext('deps:' || sqlc.arg(type)::text));

-- name: NextWorkItemNumber :one
UPDATE projects SET wi_counter = wi_counter + 1 WHERE id = $1 RETURNING wi_counter;

-- name: CreateWorkItem :one
INSERT INTO work_items (project_id, type_id, status_id, priority_id, parent_id, sequence_number, title,
                        description, description_text, author_id, assignee_id, start_date, due_date,
                        estimated_hours, remaining_hours, done_ratio, position, depth, closed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING *;

-- name: GetWorkItem :one
SELECT * FROM work_items WHERE id = $1;

-- name: GetWorkItemForUpdate :one
SELECT * FROM work_items WHERE id = $1 FOR UPDATE;

-- name: GetWorkItemByKey :one
SELECT w.* FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE p.identifier = $1 AND w.sequence_number = $2;

-- name: UpdateWorkItem :one
UPDATE work_items
SET type_id = $3, status_id = $4, priority_id = $5, parent_id = $6, title = $7, description = $8,
    description_text = $9, assignee_id = $10, start_date = $11, due_date = $12, estimated_hours = $13,
    remaining_hours = $14, done_ratio = $15, position = $16, depth = $17, closed_at = $18,
    version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING *;

-- name: RefreshWorkItemUp :exec
SELECT work_item_refresh_up($1);

-- name: ShiftWorkItemDepths :exec
WITH RECURSIVE tree AS (
    SELECT w.id FROM work_items w WHERE w.id = sqlc.arg(id)::uuid
    UNION ALL
    SELECT c.id FROM work_items c JOIN tree ON c.parent_id = tree.id
)
UPDATE work_items SET depth = depth + sqlc.arg(delta)::int FROM tree WHERE work_items.id = tree.id;

-- name: WorkItemSubtreeHeight :one
WITH RECURSIVE tree AS (
    SELECT w.id, 0 AS level FROM work_items w WHERE w.id = $1
    UNION ALL
    SELECT c.id, tree.level + 1 FROM work_items c JOIN tree ON c.parent_id = tree.id WHERE tree.level < 20
)
SELECT max(level)::int FROM tree;

-- name: WorkItemIsDescendant :one
-- Whether candidate is the work item or one of its descendants.
WITH RECURSIVE tree AS (
    SELECT w.id FROM work_items w WHERE w.id = sqlc.arg(work_item_id)::uuid
    UNION ALL
    SELECT c.id FROM work_items c JOIN tree ON c.parent_id = tree.id
)
SELECT EXISTS (SELECT 1 FROM tree WHERE tree.id = sqlc.arg(candidate)::uuid);

-- name: WorkItemAncestorIDs :many
-- The ancestors of a work item, nearest first (RF-79).
WITH RECURSIVE up AS (
    SELECT w.parent_id AS id, 1 AS level FROM work_items w WHERE w.id = $1
    UNION ALL
    SELECT p.parent_id, up.level + 1 FROM work_items p JOIN up ON p.id = up.id WHERE up.level < 20
)
SELECT up.id::uuid FROM up WHERE up.id IS NOT NULL ORDER BY up.level;

-- name: CountWorkItemChildren :one
SELECT count(*) FROM work_items WHERE parent_id = $1;

-- name: DeleteWorkItem :exec
DELETE FROM work_items WHERE id = $1;

-- name: WorkItemPeople :many
SELECT id, name FROM users WHERE id = ANY (sqlc.arg(ids)::uuid[]);

-- name: WorkItemRefs :many
-- Key and title of some work items, for parents and dependencies.
SELECT w.id, w.title, w.status_id, w.project_id, w.start_date, w.due_date, p.identifier, w.sequence_number
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.id = ANY (sqlc.arg(ids)::uuid[]);

-- name: StatusIsClosed :one
SELECT is_closed FROM statuses WHERE id = $1;

-- Dependencias ---------------------------------------------------------------

-- name: CreateDependency :one
INSERT INTO work_item_dependencies (from_id, to_id, type, lag_days, created_by)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetDependency :one
SELECT * FROM work_item_dependencies WHERE id = $1;

-- name: DeleteDependency :exec
DELETE FROM work_item_dependencies WHERE id = $1;

-- name: ListDependencies :many
-- Every dependency where the work item is source or target.
SELECT * FROM work_item_dependencies WHERE from_id = $1 OR to_id = $1 ORDER BY created_at;

-- name: DependencyReaches :one
-- Whether target is reachable from source following dependencies of one type
-- (a new source → target edge closing target → … → source would be a cycle).
WITH RECURSIVE reach AS (
    SELECT d.to_id AS id, 1 AS level FROM work_item_dependencies d
    WHERE d.from_id = sqlc.arg(source)::uuid AND d.type = sqlc.arg(type)::text
    UNION
    SELECT d.to_id, reach.level + 1 FROM work_item_dependencies d JOIN reach ON d.from_id = reach.id
    WHERE d.type = sqlc.arg(type)::text AND reach.level < 10000
)
SELECT EXISTS (SELECT 1 FROM reach WHERE reach.id = sqlc.arg(target)::uuid);

-- name: OpenBlockers :many
-- Work items that block this one and are not closed (RF-95).
SELECT w.id FROM work_item_dependencies d
JOIN work_items w ON w.id = d.from_id
JOIN statuses s ON s.id = w.status_id
WHERE d.to_id = $1 AND d.type = 'blocks' AND NOT s.is_closed;

-- Árbol ------------------------------------------------------------------------

-- name: ProjectWorkItemTree :many
-- Every work item of a project as a flat list (RF-130).
SELECT w.id, w.parent_id, w.sequence_number, w.title, w.type_id, w.status_id, w.priority_id,
       w.assignee_id, u.name AS assignee_name, w.derived_done_ratio, w.agg_estimated, w.agg_remaining,
       w.depth, w.position, w.due_date
FROM work_items w LEFT JOIN users u ON u.id = w.assignee_id
WHERE w.project_id = $1
ORDER BY w.depth, w.position, w.sequence_number;

-- name: WorkItemDescendants :many
-- Every descendant of a work item, in one query (RF-79).
WITH RECURSIVE tree AS (
    SELECT c.id FROM work_items c WHERE c.parent_id = $1
    UNION ALL
    SELECT c.id FROM work_items c JOIN tree ON c.parent_id = tree.id
)
SELECT w.id, w.parent_id, w.sequence_number, w.title, w.type_id, w.status_id, w.priority_id,
       w.assignee_id, u.name AS assignee_name, w.derived_done_ratio, w.agg_estimated, w.agg_remaining,
       w.depth, w.position, w.due_date
FROM work_items w JOIN tree ON tree.id = w.id LEFT JOIN users u ON u.id = w.assignee_id
ORDER BY w.depth, w.position, w.sequence_number;
