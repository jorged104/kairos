-- Pages of spec 005. Saving compares only page_version (RF-34, RF-59); the
-- trigger bumps it when the description changes.

-- name: UpdateWorkItemPage :one
UPDATE work_items SET description = $3, description_text = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND page_version = $2
RETURNING *;

-- name: UpdateMilestonePage :one
UPDATE milestones SET description = $3, description_text = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND page_version = $2
RETURNING *;

-- name: UpdateProjectPage :one
UPDATE projects SET description = $3, description_text = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND page_version = $2
RETURNING *;

-- name: CreatePageFile :one
INSERT INTO page_files (project_id, owner_kind, owner_id, kind, name, content_type, size_bytes, storage, s3_key, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: CreatePageFileData :exec
INSERT INTO page_file_data (file_id, data) VALUES ($1, $2);

-- name: GetPageFile :one
SELECT * FROM page_files WHERE id = $1;

-- name: GetPageFileData :one
SELECT data FROM page_file_data WHERE file_id = $1;

-- name: ListOwnerPageFiles :many
-- Every file of the owner, marked ones too: a page may bring one back (RF-50).
SELECT * FROM page_files WHERE owner_kind = $1 AND owner_id = $2 ORDER BY created_at;

-- name: SyncPageFiles :exec
-- The files of the saved page are live; the rest of the owner's are marked (RF-49, RF-50).
UPDATE page_files
SET deleted_at = CASE WHEN id = ANY(@live::uuid[]) THEN NULL ELSE coalesce(deleted_at, now()) END
WHERE owner_kind = @owner_kind AND owner_id = @owner_id;

-- name: MarkOwnerPageFilesDeleted :exec
UPDATE page_files SET deleted_at = now() WHERE owner_kind = $1 AND owner_id = $2 AND deleted_at IS NULL;

-- name: CopyPageFile :one
-- The copy of a template's file for the new owner (RF-53); the content is copied apart.
INSERT INTO page_files (project_id, owner_kind, owner_id, kind, name, content_type, size_bytes, storage, s3_key, created_by)
SELECT @project_id, owner_kind, @owner_id, kind, name, content_type, size_bytes, storage, sqlc.narg(s3_key), @created_by
FROM page_files f WHERE f.id = @id
RETURNING *;

-- name: CopyPageFileData :exec
INSERT INTO page_file_data (file_id, data) SELECT @new_id::uuid, d.data FROM page_file_data d WHERE d.file_id = @old_id::uuid;
