-- +goose Up
-- Spec 022: tareas sin proyecto. Solo aditiva: la versión anterior de la API
-- no lee nada de esto.

-- The hidden project that holds the work items without a project, one at
-- most; it is created with the first of them.
ALTER TABLE projects ADD COLUMN is_inbox boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX projects_one_inbox ON projects (is_inbox) WHERE is_inbox;

-- The team a work item without a project is for, and how: one person takes
-- it, or each member does their own as a child of the team's (RF-7 to RF-9).
ALTER TABLE work_items
    ADD COLUMN team_id   uuid REFERENCES teams (id),
    ADD COLUMN team_mode text CHECK (team_mode IN ('claim', 'each')),
    ADD CHECK ((team_id IS NULL) = (team_mode IS NULL));
CREATE INDEX work_items_team_idx ON work_items (team_id) WHERE team_id IS NOT NULL;

-- +goose Down
DROP INDEX work_items_team_idx;
ALTER TABLE work_items DROP COLUMN team_mode, DROP COLUMN team_id;
DROP INDEX projects_one_inbox;
ALTER TABLE projects DROP COLUMN is_inbox;
