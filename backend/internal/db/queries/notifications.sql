-- Notificaciones y seguidores de tareas (spec 017) --------------------------------

-- name: GetFollow :one
SELECT following FROM work_item_follows WHERE work_item_id = $1 AND user_id = $2;

-- name: SetFollow :exec
INSERT INTO work_item_follows (work_item_id, user_id, following) VALUES ($1, $2, $3)
ON CONFLICT (work_item_id, user_id) DO UPDATE SET following = EXCLUDED.following, updated_at = now();

-- name: FollowWorkItem :exec
-- The assignees just added follow again, even after «stop following» (RF-5).
INSERT INTO work_item_follows (work_item_id, user_id, following)
SELECT sqlc.arg(work_item_id)::uuid, unnest(sqlc.arg(user_ids)::uuid[]), true
ON CONFLICT (work_item_id, user_id) DO UPDATE SET following = true, updated_at = now();

-- name: WorkItemFollows :many
-- Everybody with a follow state on the work item: followers and those who stopped.
SELECT user_id, following FROM work_item_follows WHERE work_item_id = $1;

-- name: ThreadParticipants :many
-- Who wrote the root or a reply of a thread.
SELECT DISTINCT author_id FROM work_item_comments WHERE id = sqlc.arg(thread_id)::uuid OR thread_id = sqlc.arg(thread_id)::uuid;

-- name: CommentWorkItemProject :one
SELECT w.project_id, p.is_template FROM work_items w JOIN projects p ON p.id = w.project_id WHERE w.id = $1;

-- name: MarkNotified :execrows
INSERT INTO notified_events (event_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UpsertNotification :one
-- While the person has not read it, the events of the element join one
-- notification (RF-21, RF-22).
INSERT INTO notifications (user_id, subject_type, subject_id, project_id, actors, mention, last_kind, last_data, via, updated_at)
VALUES (sqlc.arg(user_id)::uuid, sqlc.arg(subject_type)::text, sqlc.arg(subject_id)::uuid, sqlc.arg(project_id)::uuid,
        ARRAY[sqlc.arg(actor)::uuid], sqlc.arg(mention)::boolean, sqlc.arg(last_kind)::text, sqlc.arg(last_data)::jsonb,
        sqlc.narg(via)::jsonb, sqlc.arg(at)::timestamptz)
ON CONFLICT (user_id, subject_type, subject_id) WHERE read_at IS NULL DO UPDATE SET
    actors     = array_prepend(sqlc.arg(actor)::uuid, array_remove(notifications.actors, sqlc.arg(actor)::uuid)),
    events     = notifications.events + 1,
    mention    = notifications.mention OR EXCLUDED.mention,
    last_kind  = EXCLUDED.last_kind,
    last_data  = EXCLUDED.last_data,
    via        = EXCLUDED.via,
    updated_at = greatest(notifications.updated_at, EXCLUDED.updated_at)
RETURNING id;

-- name: ListNotifications :many
-- A person's notifications whose element still exists (RF-45), unread first
-- (RF-27). The excerpt of a deleted comment is not shown.
SELECT n.id, n.subject_type, n.subject_id, n.project_id, n.actors, n.events, n.mention, n.last_kind, n.last_data, n.via,
       n.read_at, n.updated_at, p.name AS project_name, p.identifier AS project_identifier,
       coalesce(w.title, m.name, p.name)::text AS title, w.sequence_number AS sequence_number,
       (n.last_data ? 'comment_id' AND NOT EXISTS (
           SELECT 1 FROM work_item_comments c WHERE c.id = (n.last_data->>'comment_id')::uuid AND c.deleted_at IS NULL
       ))::boolean AS comment_gone
FROM notifications n
JOIN projects p ON p.id = n.project_id
LEFT JOIN work_items w ON n.subject_type = 'work_item' AND w.id = n.subject_id
LEFT JOIN milestones m ON n.subject_type = 'milestone' AND m.id = n.subject_id
WHERE n.user_id = sqlc.arg(user_id)::uuid
  AND (n.subject_type <> 'work_item' OR w.id IS NOT NULL)
  AND (n.subject_type <> 'milestone' OR m.id IS NOT NULL)
  AND (NOT sqlc.arg(unread_only)::boolean OR n.read_at IS NULL)
  AND (sqlc.narg(id)::uuid IS NULL OR n.id = sqlc.narg(id)::uuid)
ORDER BY (n.read_at IS NOT NULL), n.updated_at DESC, n.id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountNotifications :one
SELECT count(*) FROM notifications n
LEFT JOIN work_items w ON n.subject_type = 'work_item' AND w.id = n.subject_id
LEFT JOIN milestones m ON n.subject_type = 'milestone' AND m.id = n.subject_id
WHERE n.user_id = sqlc.arg(user_id)::uuid
  AND (n.subject_type <> 'work_item' OR w.id IS NOT NULL)
  AND (n.subject_type <> 'milestone' OR m.id IS NOT NULL)
  AND (NOT sqlc.arg(unread_only)::boolean OR n.read_at IS NULL);

-- name: UserNames :many
SELECT id, name FROM users WHERE id = ANY (sqlc.arg(ids)::uuid[]);

-- name: SetNotificationRead :execrows
UPDATE notifications SET read_at = CASE WHEN sqlc.arg(read)::boolean THEN now() END
WHERE id = sqlc.arg(id)::uuid AND user_id = sqlc.arg(user_id)::uuid;

-- name: NotificationExists :one
SELECT EXISTS (SELECT 1 FROM notifications WHERE id = sqlc.arg(id)::uuid AND user_id = sqlc.arg(user_id)::uuid);

-- name: ReadAllNotifications :exec
UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL;

-- name: PruneNotifications :exec
-- 90 days after their last event (RF-44).
DELETE FROM notifications WHERE updated_at < sqlc.arg(before)::timestamptz;

-- name: PruneNotifiedEvents :exec
DELETE FROM notified_events WHERE created_at < sqlc.arg(before)::timestamptz;
