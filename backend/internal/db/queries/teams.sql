-- name: CreateTeam :one
INSERT INTO teams (name, name_normalized, parent_id, external_ref)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTeam :one
SELECT * FROM teams WHERE id = $1;

-- name: ListTeams :many
SELECT * FROM teams ORDER BY name;

-- name: UpdateTeam :one
UPDATE teams
SET name = $2, name_normalized = $3, parent_id = $4, external_ref = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetTeamArchived :one
UPDATE teams
SET archived_at = CASE WHEN sqlc.arg(archived)::boolean THEN now() ELSE NULL END, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountActiveChildren :one
SELECT count(*) FROM teams WHERE parent_id = $1 AND archived_at IS NULL;

-- name: TeamDescendantIDs :many
-- The team itself and every subteam at any depth.
WITH RECURSIVE tree AS (
    SELECT t.id FROM teams t WHERE t.id = $1
    UNION
    SELECT c.id FROM teams c JOIN tree ON c.parent_id = tree.id
)
SELECT tree.id FROM tree;

-- name: LeaderScopeTeamIDs :many
-- Teams a user leads plus all their non-archived descendants (RF-90d, RF-91).
WITH RECURSIVE tree AS (
    SELECT t.id FROM teams t
    JOIN team_leaders l ON l.team_id = t.id
    WHERE l.user_id = $1 AND t.archived_at IS NULL
    UNION
    SELECT c.id FROM teams c JOIN tree ON c.parent_id = tree.id
    WHERE c.archived_at IS NULL
)
SELECT tree.id FROM tree;

-- name: ListTeamLeaders :many
SELECT l.team_id, l.user_id, u.name
FROM team_leaders l JOIN users u ON u.id = l.user_id
ORDER BY u.name;

-- name: CountTeamMembers :many
SELECT team_id, count(*) AS members FROM team_members GROUP BY team_id;

-- name: AddTeamLeader :exec
INSERT INTO team_leaders (team_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveTeamLeader :execrows
DELETE FROM team_leaders WHERE team_id = $1 AND user_id = $2;

-- name: ListUserTeams :many
SELECT t.id, t.name, t.parent_id, t.archived_at, m.source
FROM team_members m JOIN teams t ON t.id = m.team_id
WHERE m.user_id = $1
ORDER BY t.name;

-- name: ListTeamsForUsers :many
SELECT m.user_id, t.id, t.name, m.source
FROM team_members m JOIN teams t ON t.id = m.team_id
WHERE m.user_id = ANY(sqlc.arg(user_ids)::uuid[])
ORDER BY t.name;

-- name: GetTeamMember :one
SELECT * FROM team_members WHERE team_id = $1 AND user_id = $2;

-- name: AddManualTeamMember :exec
INSERT INTO team_members (team_id, user_id, source)
VALUES ($1, $2, 'manual')
ON CONFLICT (team_id, user_id) DO UPDATE SET source = 'manual';

-- name: AddIdpTeamMember :execrows
INSERT INTO team_members (team_id, user_id, source)
VALUES ($1, $2, 'idp')
ON CONFLICT (team_id, user_id) DO NOTHING;

-- name: DeleteTeamMember :execrows
DELETE FROM team_members WHERE team_id = $1 AND user_id = $2;
