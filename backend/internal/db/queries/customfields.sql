-- name: ListCustomFields :many
SELECT * FROM custom_fields ORDER BY entity, name;

-- name: GetCustomField :one
SELECT * FROM custom_fields WHERE id = $1;

-- name: CreateCustomField :one
INSERT INTO custom_fields (entity, name, name_normalized, type, required, options)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: UpdateCustomField :one
UPDATE custom_fields
SET name = $2, name_normalized = $3, type = $4, required = $5, options = $6, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: SetCustomFieldRetired :one
UPDATE custom_fields
SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: CustomFieldHasValues :one
SELECT (EXISTS (SELECT 1 FROM project_custom_values p WHERE p.field_id = sqlc.arg(id)::uuid)
    OR EXISTS (SELECT 1 FROM work_item_custom_values w WHERE w.field_id = sqlc.arg(id)::uuid))::boolean AS used;

-- name: ListCustomFieldTypes :many
SELECT * FROM custom_field_work_item_types;

-- name: DeleteCustomFieldTypes :exec
DELETE FROM custom_field_work_item_types WHERE field_id = $1;

-- name: AddCustomFieldType :exec
INSERT INTO custom_field_work_item_types (field_id, type_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListProjectCustomFieldIDs :many
SELECT field_id FROM project_custom_fields WHERE project_id = $1;

-- name: DeleteProjectCustomFields :exec
DELETE FROM project_custom_fields WHERE project_id = $1;

-- name: AddProjectCustomField :exec
INSERT INTO project_custom_fields (project_id, field_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListProjectCustomValues :many
SELECT * FROM project_custom_values WHERE project_id = $1;

-- name: UpsertProjectCustomValue :exec
INSERT INTO project_custom_values (project_id, field_id, value) VALUES ($1, $2, $3)
ON CONFLICT (project_id, field_id) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: DeleteProjectCustomValue :exec
DELETE FROM project_custom_values WHERE project_id = $1 AND field_id = $2;

-- name: ListWorkItemCustomValues :many
SELECT * FROM work_item_custom_values WHERE work_item_id = $1;

-- name: UpsertWorkItemCustomValue :exec
INSERT INTO work_item_custom_values (work_item_id, field_id, value) VALUES ($1, $2, $3)
ON CONFLICT (work_item_id, field_id) DO UPDATE SET value = EXCLUDED.value, updated_at = now();

-- name: DeleteWorkItemCustomValue :exec
DELETE FROM work_item_custom_values WHERE work_item_id = $1 AND field_id = $2;
