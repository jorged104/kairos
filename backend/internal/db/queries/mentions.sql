-- Mentions of the pages of spec 005 (RF-25 to RF-30).

-- name: MentionedUsers :many
SELECT id, name, status FROM users WHERE id = ANY(@ids::uuid[]);

-- name: MentionedWorkItems :many
SELECT w.id, p.identifier, w.sequence_number, w.title, (p.is_template OR p.is_inbox)::bool AS is_template,
       p.id AS project_id, p.team_id, p.owner_id
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE w.id = ANY(@ids::uuid[]);

-- name: MentionedMilestones :many
SELECT m.id, m.name, p.is_template, p.id AS project_id, p.team_id, p.owner_id
FROM milestones m JOIN projects p ON p.id = m.project_id
WHERE m.id = ANY(@ids::uuid[]);

-- name: MentionedProjects :many
SELECT id, identifier, name, (is_template OR is_inbox)::bool AS is_template, team_id, owner_id FROM projects WHERE id = ANY(@ids::uuid[]);

-- name: SearchMentionWorkItems :many
-- By key (KAI-12 or its start) or by title; templates only for who may use
-- them, and only projects the user sees (spec 023: all, or seen, or own).
SELECT w.id, p.identifier, w.sequence_number, w.title
FROM work_items w JOIN projects p ON p.id = w.project_id
WHERE (NOT p.is_template OR @templates::bool) AND NOT p.is_inbox
  AND (p.is_template OR @see_all::bool OR p.id = ANY(@seen::uuid[]) OR p.owner_id = @me::uuid OR p.team_id = ANY(@seen_teams::uuid[]))
  AND ((p.identifier || '-' || w.sequence_number) ILIKE @prefix::text OR w.title ILIKE @contains::text)
ORDER BY p.identifier, w.sequence_number
LIMIT 10;

-- name: SearchMentionMilestones :many
SELECT m.id, m.name, p.identifier
FROM milestones m JOIN projects p ON p.id = m.project_id
WHERE (NOT p.is_template OR @templates::bool) AND m.name ILIKE @contains::text
  AND (p.is_template OR @see_all::bool OR p.id = ANY(@seen::uuid[]) OR p.owner_id = @me::uuid OR p.team_id = ANY(@seen_teams::uuid[]))
ORDER BY m.name
LIMIT 10;

-- name: SearchMentionProjects :many
SELECT p.id, p.identifier, p.name
FROM projects p
WHERE (NOT p.is_template OR @templates::bool) AND NOT p.is_inbox AND (p.identifier ILIKE @prefix::text OR p.name ILIKE @contains::text)
  AND (p.is_template OR @see_all::bool OR p.id = ANY(@seen::uuid[]) OR p.owner_id = @me::uuid OR p.team_id = ANY(@seen_teams::uuid[]))
ORDER BY p.name
LIMIT 10;
