-- Reuniones y sus plantillas (spec 018) ------------------------------------------

-- name: CreateMeeting :one
INSERT INTO meetings (owner_type, owner_id, project_id, title, starts_at, location, description, description_text, template_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetMeeting :one
SELECT * FROM meetings WHERE id = $1;

-- name: GetMeetingForUpdate :one
SELECT * FROM meetings WHERE id = $1 FOR UPDATE;

-- name: UpdateMeeting :one
UPDATE meetings SET title = $3, starts_at = $4, location = $5, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING *;

-- name: UpdateMeetingPage :one
UPDATE meetings SET description = $3, description_text = $4, version = version + 1, updated_at = now()
WHERE id = $1 AND page_version = $2
RETURNING *;

-- name: DeleteMeeting :exec
DELETE FROM meetings WHERE id = $1;

-- name: ListMeetings :many
-- The upcoming ones nearest first, the past ones latest first (RF-38, RF-39).
SELECT m.id, m.title, m.starts_at, m.location,
       (SELECT count(*) FROM work_items w WHERE w.meeting_id = m.id) AS tasks
FROM meetings m
WHERE m.owner_type = sqlc.arg(owner_type)::text AND m.owner_id = sqlc.arg(owner_id)::uuid
  AND (m.starts_at >= sqlc.arg(now)::timestamptz) = sqlc.arg(upcoming)::boolean
ORDER BY CASE WHEN sqlc.arg(upcoming)::boolean THEN m.starts_at END ASC,
         CASE WHEN NOT sqlc.arg(upcoming)::boolean THEN m.starts_at END DESC, m.id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountMeetings :one
SELECT count(*) FROM meetings m
WHERE m.owner_type = sqlc.arg(owner_type)::text AND m.owner_id = sqlc.arg(owner_id)::uuid
  AND (m.starts_at >= sqlc.arg(now)::timestamptz) = sqlc.arg(upcoming)::boolean;

-- name: MeetingTasks :many
-- The work items that came from a meeting (RF-27).
SELECT w.id, p.identifier, w.sequence_number, w.title, w.status_id
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.meeting_id = $1
ORDER BY w.sequence_number;

-- name: ChooseMeetingTemplates :many
-- What a new meeting of a project may start from: the active global ones,
-- then the project's (RF-34).
SELECT * FROM meeting_templates
WHERE (project_id IS NULL AND retired_at IS NULL) OR project_id = sqlc.arg(project_id)::uuid
ORDER BY project_id NULLS FIRST, position, name;

-- name: GlobalMeetingTemplates :many
SELECT * FROM meeting_templates WHERE project_id IS NULL ORDER BY position, name;

-- name: GetMeetingTemplate :one
SELECT * FROM meeting_templates WHERE id = $1;

-- name: GetMeetingTemplateForUpdate :one
SELECT * FROM meeting_templates WHERE id = $1 FOR UPDATE;

-- name: CreateMeetingTemplate :one
INSERT INTO meeting_templates (project_id, name, name_normalized, title, location, page, position)
VALUES (sqlc.narg(project_id)::uuid, sqlc.arg(name)::text, sqlc.arg(name_normalized)::text, sqlc.arg(title)::text,
        sqlc.arg(location)::text, sqlc.narg(page)::jsonb,
        coalesce((SELECT max(t.position) + 1 FROM meeting_templates t
                  WHERE t.project_id IS NOT DISTINCT FROM sqlc.narg(project_id)::uuid), 0))
RETURNING *;

-- name: UpdateMeetingTemplate :one
UPDATE meeting_templates SET name = $2, name_normalized = $3, title = $4, location = $5, page = $6, position = $7, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetMeetingTemplateRetired :one
UPDATE meeting_templates SET retired_at = CASE WHEN sqlc.arg(retired)::boolean THEN now() END, updated_at = now()
WHERE id = sqlc.arg(id)::uuid
RETURNING *;

-- name: DeleteMeetingTemplate :exec
DELETE FROM meeting_templates WHERE id = $1;

-- name: ListMeetingComments :many
-- Every comment of a meeting, as ListComments for a work item.
SELECT c.id, c.work_item_id, c.thread_id, c.author_id, a.name AS author_name, c.body, c.quote, c.anchored,
       c.created_at, c.edited_at, c.deleted_at, c.resolved_at, c.resolved_by, r.name AS resolved_by_name, c.via
FROM work_item_comments c
JOIN users a ON a.id = c.author_id
LEFT JOIN users r ON r.id = c.resolved_by
WHERE c.meeting_id = $1
ORDER BY c.created_at, c.id;

-- name: MentionedMeetings :many
SELECT m.id, m.title, p.is_template, p.id AS project_id, p.team_id, p.owner_id
FROM meetings m JOIN projects p ON p.id = m.project_id
WHERE m.id = ANY(@ids::uuid[]);

-- name: SearchMentionMeetings :many
SELECT m.id, m.title, p.identifier
FROM meetings m JOIN projects p ON p.id = m.project_id
WHERE (NOT p.is_template OR @templates::bool) AND m.title ILIKE @contains::text
  AND (p.is_template OR @see_all::bool OR p.id = ANY(@seen::uuid[]) OR p.owner_id = @me::uuid OR p.team_id = ANY(@seen_teams::uuid[]))
ORDER BY m.starts_at DESC
LIMIT 10;
