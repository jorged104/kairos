-- Spec 013: Mi panel.

-- name: DashboardTasks :many
-- The work items assigned to the user in active projects that are not
-- templates, open or closed since a day (RF-10, RF-11, RF-28).
SELECT w.id, w.project_id, p.identifier, p.name AS project_name, p.team_id, p.owner_id,
       w.sequence_number, w.title, w.status_id, s.is_closed, w.due_date, w.closed_at, w.weight, w.type_id
FROM work_item_assignees a
JOIN work_items w ON w.id = a.work_item_id
JOIN projects p ON p.id = w.project_id AND p.lifecycle = 'activo' AND NOT p.is_template
JOIN statuses s ON s.id = w.status_id
WHERE a.user_id = sqlc.arg(user_id)::uuid
  AND (NOT s.is_closed OR w.closed_at >= sqlc.arg(since)::timestamptz)
ORDER BY p.name, w.sequence_number;

-- name: DashboardProjects :many
-- The active projects the user owns, is a member of (named, or a participant
-- of the project's team, spec 008) or has work items in (RF-23).
SELECT p.id, p.identifier, p.name, p.team_id, p.owner_id
FROM projects p
WHERE p.lifecycle = 'activo' AND NOT p.is_template
  AND (p.owner_id = sqlc.arg(user_id)::uuid
       OR p.team_id = ANY (sqlc.arg(team_ids)::uuid[])
       OR EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = p.id AND m.user_id = sqlc.arg(user_id)::uuid)
       OR EXISTS (SELECT 1 FROM work_item_assignees a JOIN work_items w ON w.id = a.work_item_id
                  WHERE w.project_id = p.id AND a.user_id = sqlc.arg(user_id)::uuid))
ORDER BY p.name, p.identifier;
