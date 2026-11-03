-- +goose Up
-- Spec 008: papeles por proyecto (RF-33 a RF-45), capacidad semanal (RF-46),
-- vistas guardadas (RF-23 a RF-31), límites del kanban (RF-66) y registro diario
-- del avance (RF-12). Solo aditiva.

-- The roles set on purpose; the owner (PM) and the team's participants
-- (member) have theirs without a row (RF-34 to RF-36).
CREATE TABLE project_members (
    project_id uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES users (id),
    role       text        NOT NULL CHECK (role IN ('pm', 'tech_lead', 'member', 'observer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX project_members_user_idx ON project_members (user_id);

ALTER TABLE users ADD COLUMN weekly_capacity int CHECK (weekly_capacity BETWEEN 0 AND 1000);

-- A personal view has its owner; a shared one has none (RF-24, RF-25).
CREATE TABLE project_views (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    owner_id        uuid        REFERENCES users (id),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    config          jsonb       NOT NULL DEFAULT '{}',
    created_by      uuid        NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX project_views_personal_name_idx ON project_views (project_id, owner_id, name_normalized) WHERE owner_id IS NOT NULL;
CREATE UNIQUE INDEX project_views_shared_name_idx ON project_views (project_id, name_normalized) WHERE owner_id IS NULL;

CREATE TABLE project_column_limits (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    status_id  uuid NOT NULL REFERENCES statuses (id),
    wip_limit  int  NOT NULL CHECK (wip_limit BETWEEN 1 AND 999),
    PRIMARY KEY (project_id, status_id)
);

CREATE TABLE project_progress_daily (
    project_id uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    day        date        NOT NULL,
    progress   smallint    NOT NULL CHECK (progress BETWEEN 0 AND 100),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, day)
);

-- +goose Down
DROP TABLE project_progress_daily;
DROP TABLE project_column_limits;
DROP TABLE project_views;
ALTER TABLE users DROP COLUMN weekly_capacity;
DROP TABLE project_members;
