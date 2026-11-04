-- +goose Up
-- Spec 012: estados de proyecto (RF-1 a RF-16) y vistas de la lista de
-- proyectos (RF-69 a RF-80). Solo aditiva: projects.status_id admite nulos para
-- que la versión anterior de la API siga creando proyectos; un nulo se lee
-- como el estado inicial.
CREATE TABLE project_statuses (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    color           text        NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9a-f]{6}$'),
    is_closed       boolean     NOT NULL DEFAULT false,
    is_initial      boolean     NOT NULL DEFAULT false,
    is_closing      boolean     NOT NULL DEFAULT false,
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT (is_initial AND is_closed)),
    CHECK (NOT is_closing OR is_closed),
    CHECK (NOT ((is_initial OR is_closing) AND retired_at IS NOT NULL))
);
CREATE UNIQUE INDEX project_statuses_active_name_idx ON project_statuses (name_normalized) WHERE retired_at IS NULL;
CREATE UNIQUE INDEX project_statuses_initial_idx ON project_statuses ((true)) WHERE is_initial;
CREATE UNIQUE INDEX project_statuses_closing_idx ON project_statuses ((true)) WHERE is_closing;

INSERT INTO project_statuses (name, name_normalized, color, is_closed, is_initial, is_closing, position) VALUES
    ('Propuesta', 'propuesta', '#94a3b8', false, true, false, 1),
    ('Planificación', 'planificación', '#6366f1', false, false, false, 2),
    ('En ejecución', 'en ejecución', '#2563eb', false, false, false, 3),
    ('En pausa', 'en pausa', '#d97706', false, false, false, 4),
    ('Cerrado', 'cerrado', '#16a34a', true, false, true, 5);

ALTER TABLE projects ADD COLUMN status_id uuid REFERENCES project_statuses (id);
UPDATE projects SET status_id = (SELECT id FROM project_statuses WHERE name_normalized = 'en ejecución')
WHERE lifecycle IN ('activo', 'archivado');
UPDATE projects SET status_id = (SELECT id FROM project_statuses WHERE is_closing)
WHERE lifecycle = 'cerrado';
CREATE INDEX projects_status_idx ON projects (status_id);

CREATE TABLE project_list_views (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        uuid        REFERENCES users (id),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    config          jsonb       NOT NULL DEFAULT '{}',
    created_by      uuid        NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX project_list_views_personal_name_idx ON project_list_views (owner_id, name_normalized) WHERE owner_id IS NOT NULL;
CREATE UNIQUE INDEX project_list_views_shared_name_idx ON project_list_views (name_normalized) WHERE owner_id IS NULL;

-- +goose Down
DROP TABLE project_list_views;
ALTER TABLE projects DROP COLUMN status_id;
DROP TABLE project_statuses;
