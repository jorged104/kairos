-- Mentions of the pages of spec 005 (RF-25 to RF-30).

-- name: MentionedUsers :many
SELECT id, name, status FROM users WHERE id = ANY(@ids::uuid[]);

-- name: MentionedWorkItems :many
SELECT w.id, p.identifier, w.sequence_number, w.title, p.is_template
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.id = ANY(@ids::uuid[]);

-- name: MentionedMilestones :many
SELECT m.id, m.name, p.is_template
FROM milestones m JOIN projects p ON p.id = m.project_id
WHERE m.id = ANY(@ids::uuid[]);

-- name: MentionedProjects :many
SELECT id, identifier, name, is_template FROM projects WHERE id = ANY(@ids::uuid[]);

-- name: SearchMentionWorkItems :many
-- By key (KAI-12 or its start) or by title; templates only for who may use them.
SELECT w.id, p.identifier, w.sequence_number, w.title
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE (NOT p.is_template OR @templates::bool)
  AND ((p.identifier || '-' || w.sequence_number) ILIKE @prefix::text OR w.title ILIKE @contains::text)
ORDER BY p.identifier, w.sequence_number
LIMIT 10;

-- name: SearchMentionMilestones :many
SELECT m.id, m.name, p.identifier
FROM milestones m JOIN projects p ON p.id = m.project_id
WHERE (NOT p.is_template OR @templates::bool) AND m.name ILIKE @contains::text
ORDER BY m.name
LIMIT 10;

-- name: SearchMentionProjects :many
SELECT id, identifier, name
FROM projects
WHERE (NOT is_template OR @templates::bool) AND (identifier ILIKE @prefix::text OR name ILIKE @contains::text)
ORDER BY name
LIMIT 10;
