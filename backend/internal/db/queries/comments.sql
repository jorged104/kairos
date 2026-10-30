-- Comentarios de work items (spec 006) ------------------------------------------

-- name: CreateComment :one
-- An anchored thread brings its own id, the one its mark in the page uses (RF-45).
INSERT INTO work_item_comments (id, work_item_id, thread_id, author_id, body, body_text, quote, anchored)
VALUES (coalesce(sqlc.narg(id)::uuid, gen_random_uuid()), sqlc.arg(work_item_id)::uuid, sqlc.narg(thread_id)::uuid,
        sqlc.arg(author_id)::uuid, sqlc.arg(body)::jsonb, sqlc.arg(body_text)::text, sqlc.narg(quote)::text,
        sqlc.arg(anchored)::boolean)
RETURNING *;

-- name: CommentExists :one
SELECT EXISTS (SELECT 1 FROM work_item_comments WHERE id = $1);

-- name: GetCommentForUpdate :one
SELECT * FROM work_item_comments WHERE id = $1 FOR UPDATE;

-- name: ListComments :many
-- Every comment of a work item, roots and replies, with the names to show.
SELECT c.id, c.work_item_id, c.thread_id, c.author_id, a.name AS author_name, c.body, c.quote, c.anchored,
       c.created_at, c.edited_at, c.deleted_at, c.resolved_at, c.resolved_by, r.name AS resolved_by_name
FROM work_item_comments c
JOIN users a ON a.id = c.author_id
LEFT JOIN users r ON r.id = c.resolved_by
WHERE c.work_item_id = $1
ORDER BY c.created_at, c.id;

-- name: UpdateCommentBody :one
UPDATE work_item_comments SET body = $2, body_text = $3, edited_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteComment :exec
DELETE FROM work_item_comments WHERE id = $1;

-- name: BlankComment :exec
-- A root with replies stays as «Comentario eliminado» (RF-33).
UPDATE work_item_comments SET body = NULL, body_text = '', deleted_at = now() WHERE id = $1;

-- name: CountReplies :one
SELECT count(*) FROM work_item_comments WHERE thread_id = $1;

-- name: SetThreadResolved :one
UPDATE work_item_comments
SET resolved_at = CASE WHEN sqlc.arg(resolved)::boolean THEN now() END,
    resolved_by = CASE WHEN sqlc.arg(resolved)::boolean THEN sqlc.narg(resolved_by)::uuid END
WHERE id = sqlc.arg(id)::uuid RETURNING *;
