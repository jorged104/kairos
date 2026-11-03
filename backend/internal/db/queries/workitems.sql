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
                        estimated_hours, remaining_hours, done_ratio, position, depth, closed_at, weight, size_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
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
    remaining_hours = $14, done_ratio = $15, position = $16, depth = $17, closed_at = $18, weight = $19,
    size_id = $20, version = version + 1, updated_at = now()
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

-- Asignados (spec 006) -------------------------------------------------------

-- name: ListWorkItemAssignees :many
-- The assignees of some work items, in their order (RF-12).
SELECT a.work_item_id, u.id, u.name FROM work_item_assignees a JOIN users u ON u.id = a.user_id
WHERE a.work_item_id = ANY (sqlc.arg(ids)::uuid[])
ORDER BY a.work_item_id, a.position, a.created_at;

-- name: ClearWorkItemAssignees :exec
DELETE FROM work_item_assignees WHERE work_item_id = $1;

-- name: AddWorkItemAssignees :exec
-- Writes the list before the row, so the compatibility trigger keeps it.
INSERT INTO work_item_assignees (work_item_id, user_id, position)
SELECT sqlc.arg(work_item_id)::uuid, u.id, u.pos::int - 1
FROM unnest(sqlc.arg(user_ids)::uuid[]) WITH ORDINALITY AS u(id, pos)
ON CONFLICT (work_item_id, user_id) DO UPDATE SET position = EXCLUDED.position;

-- Etiquetas (spec 007) ------------------------------------------------------------

-- name: ListWorkItemLabels :many
-- The labels of some work items, in the catalog's order (RF-42).
SELECT wl.work_item_id, l.id FROM work_item_labels wl JOIN labels l ON l.id = wl.label_id
WHERE wl.work_item_id = ANY (sqlc.arg(ids)::uuid[])
ORDER BY wl.work_item_id, l.position, l.name;

-- name: ClearWorkItemLabels :exec
DELETE FROM work_item_labels WHERE work_item_id = $1;

-- name: AddWorkItemLabels :exec
INSERT INTO work_item_labels (work_item_id, label_id)
SELECT sqlc.arg(work_item_id)::uuid, unnest(sqlc.arg(label_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: CopyWorkItemLabels :exec
-- A template copies the labels that are still active (spec 007, RF-45).
INSERT INTO work_item_labels (work_item_id, label_id)
SELECT sqlc.arg(to_id)::uuid, wl.label_id FROM work_item_labels wl JOIN labels l ON l.id = wl.label_id
WHERE wl.work_item_id = sqlc.arg(from_id)::uuid AND l.retired_at IS NULL;

-- name: ActiveLabelIDs :many
-- The ones of these labels that exist and are not retired (RF-39).
SELECT id FROM labels WHERE id = ANY (sqlc.arg(ids)::uuid[]) AND retired_at IS NULL;

-- Métricas e historial (spec 007) --------------------------------------------------

-- name: WorkItemAudit :many
-- The audit of some work items, oldest first.
SELECT id, occurred_at, actor_id, action, target_id, before, after FROM audit_log
WHERE target_type = 'work_item' AND target_id = ANY (sqlc.arg(ids)::text[])
ORDER BY id;

-- name: WorkItemHistoryPage :many
-- A batch of the history of a work item, newest first (RF-52).
SELECT a.id, a.occurred_at, a.actor_id, coalesce(u.name, '') AS actor_name, a.action, a.before, a.after
FROM audit_log a LEFT JOIN users u ON u.id = a.actor_id
WHERE a.target_type = 'work_item' AND a.target_id = sqlc.arg(target_id)::text
  AND a.action IN ('work_item.created', 'work_item.updated', 'work_item.page_updated',
                   'work_item.dependency_added', 'work_item.dependency_removed')
  AND (sqlc.narg(before_id)::bigint IS NULL OR a.id < sqlc.narg(before_id))
ORDER BY a.id DESC
LIMIT sqlc.arg(row_limit);

-- name: BlockingDependencyAudit :many
-- Who has blocked a work item at any time, from the audit of the blockers (RF-33).
SELECT id, occurred_at, action, target_id, before, after FROM audit_log
WHERE target_type = 'work_item' AND action IN ('work_item.dependency_added', 'work_item.dependency_removed')
  AND coalesce(after, before)->>'type' = 'blocks'
  AND coalesce(after, before)->>'to_id' = sqlc.arg(to_id)::text
ORDER BY id;

-- name: CurrentBlockingDependencies :many
SELECT from_id, created_at FROM work_item_dependencies WHERE to_id = $1 AND type = 'blocks';

-- name: ProjectWeight :one
SELECT coalesce(sum(weight), 0)::bigint FROM work_items WHERE project_id = $1;

-- name: ClosedStatusIDs :many
SELECT id FROM statuses WHERE is_closed;

-- name: NamesOfUsers :many
SELECT id, name FROM users WHERE id = ANY (sqlc.arg(ids)::uuid[]);

-- name: GetWorkItemSize :one
SELECT * FROM work_item_sizes WHERE id = $1;

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
-- Every work item of a project as a flat list (RF-130). The derived ratio is
-- the one weighted by weight (RF-153); derived_done_ratio is the previous one.
SELECT w.id, w.parent_id, w.sequence_number, w.title, w.type_id, w.status_id, w.priority_id,
       w.assignee_id, u.name AS assignee_name, w.weighted_done_ratio AS derived_done_ratio, w.agg_estimated, w.agg_remaining,
       w.depth, w.position, w.due_date, w.weight, w.size_id
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
       w.assignee_id, u.name AS assignee_name, w.weighted_done_ratio AS derived_done_ratio, w.agg_estimated, w.agg_remaining,
       w.depth, w.position, w.due_date, w.weight, w.size_id
FROM work_items w JOIN tree ON tree.id = w.id LEFT JOIN users u ON u.id = w.assignee_id
ORDER BY w.depth, w.position, w.sequence_number;
