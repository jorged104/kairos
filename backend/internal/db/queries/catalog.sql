-- Metodologías -----------------------------------------------------------------

-- name: ListMethodologies :many
SELECT * FROM methodologies ORDER BY position, name;

-- name: GetMethodology :one
SELECT * FROM methodologies WHERE id = $1;

-- name: CreateMethodology :one
INSERT INTO methodologies (name, name_normalized, position) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateMethodology :one
UPDATE methodologies SET name = $2, name_normalized = $3, position = $4, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: SetMethodologyRetired :one
UPDATE methodologies
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ListPhaseDefinitions :many
SELECT * FROM phase_definitions ORDER BY methodology_id, position, name;

-- name: GetPhaseDefinition :one
SELECT * FROM phase_definitions WHERE id = $1;

-- name: CreatePhaseDefinition :one
INSERT INTO phase_definitions (methodology_id, name, position, has_start_gate, has_finish_gate)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: UpdatePhaseDefinition :one
UPDATE phase_definitions
SET name = $2, position = $3, has_start_gate = $4, has_finish_gate = $5, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: SetPhaseDefinitionRetired :one
UPDATE phase_definitions
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- Tipos ----------------------------------------------------------------------

-- name: ListWorkItemTypes :many
SELECT * FROM work_item_types ORDER BY position, name;

-- name: GetWorkItemType :one
SELECT * FROM work_item_types WHERE id = $1;

-- name: CreateWorkItemType :one
-- A type without a default weight weighs 1 (RF-150).
INSERT INTO work_item_types (name, name_normalized, color, position, default_weight)
VALUES ($1, $2, $3, $4, coalesce(sqlc.narg(default_weight)::smallint, 1)) RETURNING *;

-- name: UpdateWorkItemType :one
-- A client that doesn't send the default weight keeps the current one.
UPDATE work_item_types SET name = $2, name_normalized = $3, color = $4, position = $5,
    default_weight = coalesce(sqlc.narg(default_weight)::smallint, default_weight), updated_at = now()
WHERE id = $1 RETURNING *;

-- name: SetWorkItemTypeRetired :one
UPDATE work_item_types
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- Estados ----------------------------------------------------------------------

-- name: ListStatuses :many
SELECT * FROM statuses ORDER BY position, name;

-- name: GetStatus :one
SELECT * FROM statuses WHERE id = $1;

-- name: GetStatusForUpdate :one
SELECT * FROM statuses WHERE id = $1 FOR UPDATE;

-- name: CreateStatus :one
INSERT INTO statuses (name, name_normalized, color, position, is_closed, is_default)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: UpdateStatus :one
UPDATE statuses
SET name = $2, name_normalized = $3, color = $4, position = $5, is_closed = $6, is_default = $7, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ClearDefaultStatus :exec
UPDATE statuses SET is_default = false, updated_at = now() WHERE is_default AND id <> $1;

-- name: SetStatusRetired :one
UPDATE statuses
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: StatusInUse :one
SELECT EXISTS (SELECT 1 FROM work_items WHERE status_id = $1);

-- name: GetDefaultStatus :one
SELECT * FROM statuses WHERE is_default;

-- Prioridades ------------------------------------------------------------------

-- name: ListPriorities :many
SELECT * FROM priorities ORDER BY position, name;

-- name: GetPriority :one
SELECT * FROM priorities WHERE id = $1;

-- name: CreatePriority :one
INSERT INTO priorities (name, name_normalized, color, position, is_default)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: UpdatePriority :one
UPDATE priorities
SET name = $2, name_normalized = $3, color = $4, position = $5, is_default = $6, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: ClearDefaultPriority :exec
UPDATE priorities SET is_default = false, updated_at = now() WHERE is_default AND id <> $1;

-- name: SetPriorityRetired :one
UPDATE priorities
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: GetDefaultPriority :one
SELECT * FROM priorities WHERE is_default;

-- Tallas (spec 006, RF-1 a RF-3) ---------------------------------------------------

-- name: ListWorkItemSizes :many
SELECT * FROM work_item_sizes ORDER BY position, name;

-- name: CreateWorkItemSize :one
INSERT INTO work_item_sizes (name, name_normalized, position)
VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateWorkItemSize :one
UPDATE work_item_sizes
SET name = $2, name_normalized = $3, position = $4, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: SetWorkItemSizeRetired :one
UPDATE work_item_sizes
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- Flujos -------------------------------------------------------------------------

-- name: ListTransitions :many
SELECT * FROM workflow_transitions WHERE type_id = $1 ORDER BY from_status_id, to_status_id, role NULLS FIRST;

-- name: DeleteTransitions :exec
DELETE FROM workflow_transitions WHERE type_id = $1;

-- name: CreateTransition :exec
INSERT INTO workflow_transitions (type_id, from_status_id, to_status_id, role) VALUES ($1, $2, $3, $4);

-- name: CountTransitions :one
SELECT count(*) FROM workflow_transitions WHERE type_id = $1;

-- name: TransitionExists :one
SELECT EXISTS (
    SELECT 1 FROM workflow_transitions
    WHERE type_id = $1 AND from_status_id = $2 AND to_status_id = $3
      AND (role IS NULL OR role = ANY (sqlc.arg(roles)::text[]))
);

-- name: AllowedTargetStatuses :many
-- Statuses a user with these roles can move a work item of this type to,
-- from its current status (RF-86, RF-87, RF-89).
SELECT s.id FROM statuses s
WHERE s.retired_at IS NULL AND s.id <> sqlc.arg(from_status_id)
  AND (
    NOT EXISTS (SELECT 1 FROM workflow_transitions t WHERE t.type_id = sqlc.arg(type_id))
    OR EXISTS (
        SELECT 1 FROM workflow_transitions t
        WHERE t.type_id = sqlc.arg(type_id) AND t.from_status_id = sqlc.arg(from_status_id) AND t.to_status_id = s.id
          AND (t.role IS NULL OR t.role = ANY (sqlc.arg(roles)::text[]))
    )
  )
ORDER BY s.position, s.name;
