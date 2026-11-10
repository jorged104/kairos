-- Spec 021: cronograma por persona.

-- name: PersonScheduleItems :many
-- The open work items assigned to some people, out of templates, optionally of
-- one project only (RF-1, RF-18, RF-24).
SELECT a.user_id, w.id, w.project_id, p.identifier, p.name AS project_name, w.sequence_number, w.title,
       w.start_date, w.due_date, w.weight, coalesce(w.weighted_done_ratio, w.done_ratio)::smallint AS done_ratio,
       p.is_inbox
FROM work_item_assignees a
JOIN work_items w ON w.id = a.work_item_id
JOIN statuses s ON s.id = w.status_id
JOIN projects p ON p.id = w.project_id
WHERE a.user_id = ANY (sqlc.arg(ids)::uuid[]) AND NOT s.is_closed AND NOT p.is_template
  AND (sqlc.narg(project_id)::uuid IS NULL OR w.project_id = sqlc.narg(project_id)::uuid)
ORDER BY coalesce(w.start_date, w.due_date) NULLS LAST, p.identifier, w.sequence_number;
