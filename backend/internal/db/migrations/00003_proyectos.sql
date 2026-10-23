-- +goose Up
-- Spec 004: catálogo, proyectos, fases, hitos, work items, dependencias, objetivos y eventos.

-- Catálogo ------------------------------------------------------------------

CREATE TABLE methodologies (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX methodologies_active_name_idx ON methodologies (name_normalized) WHERE retired_at IS NULL;

CREATE TABLE phase_definitions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    methodology_id  uuid        NOT NULL REFERENCES methodologies (id),
    name            text        NOT NULL,
    position        int         NOT NULL DEFAULT 0,
    has_start_gate  boolean     NOT NULL DEFAULT false,
    has_finish_gate boolean     NOT NULL DEFAULT false,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX phase_definitions_methodology_id_idx ON phase_definitions (methodology_id);

CREATE TABLE work_item_types (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    color           text        NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9a-f]{6}$'),
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX work_item_types_active_name_idx ON work_item_types (name_normalized) WHERE retired_at IS NULL;

CREATE TABLE statuses (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    color           text        NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9a-f]{6}$'),
    is_closed       boolean     NOT NULL DEFAULT false,
    is_default      boolean     NOT NULL DEFAULT false,
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT (is_default AND is_closed)),
    CHECK (NOT (is_default AND retired_at IS NOT NULL))
);
CREATE UNIQUE INDEX statuses_active_name_idx ON statuses (name_normalized) WHERE retired_at IS NULL;
CREATE UNIQUE INDEX statuses_single_default_idx ON statuses (is_default) WHERE is_default;

CREATE TABLE priorities (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    color           text        NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9a-f]{6}$'),
    is_default      boolean     NOT NULL DEFAULT false,
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (NOT (is_default AND retired_at IS NOT NULL))
);
CREATE UNIQUE INDEX priorities_active_name_idx ON priorities (name_normalized) WHERE retired_at IS NULL;
CREATE UNIQUE INDEX priorities_single_default_idx ON priorities (is_default) WHERE is_default;

CREATE TABLE workflow_transitions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type_id        uuid NOT NULL REFERENCES work_item_types (id),
    from_status_id uuid NOT NULL REFERENCES statuses (id),
    to_status_id   uuid NOT NULL REFERENCES statuses (id),
    role           text CHECK (role IN ('admin', 'lead', 'member', 'viewer')),
    CHECK (from_status_id <> to_status_id),
    UNIQUE NULLS NOT DISTINCT (type_id, from_status_id, to_status_id, role)
);

CREATE TABLE custom_fields (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entity          text        NOT NULL CHECK (entity IN ('project', 'work_item')),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    type            text        NOT NULL CHECK (type IN ('text', 'number', 'date', 'boolean', 'select')),
    required        boolean     NOT NULL DEFAULT false,
    options         jsonb       NOT NULL DEFAULT '[]',
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX custom_fields_active_name_idx ON custom_fields (entity, name_normalized) WHERE retired_at IS NULL;

-- Empty for a field means it applies to every type (RF-14).
CREATE TABLE custom_field_work_item_types (
    field_id uuid NOT NULL REFERENCES custom_fields (id) ON DELETE CASCADE,
    type_id  uuid NOT NULL REFERENCES work_item_types (id),
    PRIMARY KEY (field_id, type_id)
);

-- Proyectos -----------------------------------------------------------------

CREATE TABLE projects (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id          uuid        REFERENCES projects (id),
    template_id        uuid        REFERENCES projects (id),
    team_id            uuid        NOT NULL REFERENCES teams (id),
    owner_id           uuid        NOT NULL REFERENCES users (id),
    methodology_id     uuid        REFERENCES methodologies (id),
    identifier         text        NOT NULL UNIQUE CHECK (identifier ~ '^[A-Z][A-Z0-9]{1,9}$'),
    name               text        NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 255),
    description        jsonb,
    description_text   text        NOT NULL DEFAULT '',
    is_template        boolean     NOT NULL DEFAULT false,
    lifecycle          text        NOT NULL DEFAULT 'activo' CHECK (lifecycle IN ('activo', 'cerrado', 'archivado')),
    health             text        NOT NULL DEFAULT 'en_curso' CHECK (health IN ('en_curso', 'en_riesgo', 'fuera_de_curso')),
    health_explanation text        NOT NULL DEFAULT '',
    start_date         date,
    end_date           date,
    depth              smallint    NOT NULL DEFAULT 0 CHECK (depth BETWEEN 0 AND 4),
    wi_counter         int         NOT NULL DEFAULT 0,
    version            int         NOT NULL DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (parent_id IS NULL OR parent_id <> id),
    CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);
CREATE INDEX projects_parent_id_idx ON projects (parent_id);
CREATE INDEX projects_team_id_idx ON projects (team_id);
CREATE INDEX projects_owner_id_idx ON projects (owner_id);

CREATE TABLE project_work_item_types (
    project_id uuid NOT NULL REFERENCES projects (id),
    type_id    uuid NOT NULL REFERENCES work_item_types (id),
    PRIMARY KEY (project_id, type_id)
);

CREATE TABLE project_custom_fields (
    project_id uuid NOT NULL REFERENCES projects (id),
    field_id   uuid NOT NULL REFERENCES custom_fields (id),
    PRIMARY KEY (project_id, field_id)
);

CREATE TABLE project_custom_values (
    project_id uuid        NOT NULL REFERENCES projects (id),
    field_id   uuid        NOT NULL REFERENCES custom_fields (id),
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, field_id)
);

CREATE TABLE project_phases (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            uuid        NOT NULL REFERENCES projects (id),
    definition_id         uuid        NOT NULL REFERENCES phase_definitions (id),
    position              int         NOT NULL DEFAULT 0,
    start_date            date,
    finish_date           date,
    start_gate_passed_at  timestamptz,
    start_gate_passed_by  uuid        REFERENCES users (id),
    finish_gate_passed_at timestamptz,
    finish_gate_passed_by uuid        REFERENCES users (id),
    UNIQUE (id, project_id),
    UNIQUE (project_id, definition_id),
    CHECK (finish_date IS NULL OR start_date IS NULL OR finish_date >= start_date)
);

-- Work items ----------------------------------------------------------------

CREATE TABLE work_items (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid          NOT NULL REFERENCES projects (id),
    type_id           uuid          NOT NULL REFERENCES work_item_types (id),
    status_id         uuid          NOT NULL REFERENCES statuses (id),
    priority_id       uuid          NOT NULL REFERENCES priorities (id),
    parent_id         uuid,
    sequence_number   int           NOT NULL,
    title             text          NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 255),
    description       jsonb,
    description_text  text          NOT NULL DEFAULT '',
    author_id         uuid          NOT NULL REFERENCES users (id),
    assignee_id       uuid          REFERENCES users (id),
    start_date        date,
    due_date          date,
    estimated_hours   numeric(7, 2) CHECK (estimated_hours BETWEEN 0 AND 10000),
    remaining_hours   numeric(7, 2) CHECK (remaining_hours BETWEEN 0 AND 10000),
    done_ratio        smallint      NOT NULL DEFAULT 0 CHECK (done_ratio BETWEEN 0 AND 100),
    position          int           NOT NULL DEFAULT 0,
    depth             smallint      NOT NULL DEFAULT 0 CHECK (depth BETWEEN 0 AND 9),
    -- Subtree aggregates, the node included (plan: valores derivados).
    agg_estimated     numeric(14, 2) NOT NULL DEFAULT 0,
    agg_remaining     numeric(14, 2) NOT NULL DEFAULT 0,
    agg_count         int            NOT NULL DEFAULT 1,
    agg_ratio_sum     bigint         NOT NULL DEFAULT 0,
    agg_est_ratio_sum numeric(18, 2) NOT NULL DEFAULT 0,
    derived_done_ratio smallint GENERATED ALWAYS AS (
        CASE WHEN agg_estimated > 0 THEN round(agg_est_ratio_sum / agg_estimated)::smallint
             ELSE round(agg_ratio_sum::numeric / agg_count)::smallint END
    ) STORED,
    version           int           NOT NULL DEFAULT 1,
    closed_at         timestamptz,
    created_at        timestamptz   NOT NULL DEFAULT now(),
    updated_at        timestamptz   NOT NULL DEFAULT now(),
    UNIQUE (project_id, sequence_number),
    UNIQUE (id, project_id),
    FOREIGN KEY (parent_id, project_id) REFERENCES work_items (id, project_id),
    CHECK (parent_id IS NULL OR parent_id <> id),
    CHECK (due_date IS NULL OR start_date IS NULL OR due_date >= start_date)
);
CREATE INDEX work_items_project_parent_idx ON work_items (project_id, parent_id);
CREATE INDEX work_items_parent_id_idx ON work_items (parent_id);
CREATE INDEX work_items_assignee_id_idx ON work_items (assignee_id);
CREATE INDEX work_items_project_status_idx ON work_items (project_id, status_id);

CREATE TABLE work_item_custom_values (
    work_item_id uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    field_id     uuid        NOT NULL REFERENCES custom_fields (id),
    value        jsonb       NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (work_item_id, field_id)
);

CREATE TABLE work_item_dependencies (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    from_id    uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    to_id      uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    type       text        NOT NULL CHECK (type IN ('blocks', 'precedes', 'relates', 'duplicates')),
    lag_days   smallint    NOT NULL DEFAULT 0 CHECK (lag_days BETWEEN -365 AND 365),
    created_by uuid        NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (from_id, to_id, type),
    CHECK (from_id <> to_id),
    CHECK (type = 'precedes' OR lag_days = 0)
);
CREATE INDEX work_item_dependencies_to_id_idx ON work_item_dependencies (to_id);

-- Hitos ---------------------------------------------------------------------

CREATE TABLE milestones (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       uuid        NOT NULL REFERENCES projects (id),
    phase_id         uuid,
    name             text        NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 255),
    description      jsonb,
    description_text text        NOT NULL DEFAULT '',
    due_date         date,
    completed_on     date,
    status           text        NOT NULL DEFAULT 'abierto' CHECK (status IN ('abierto', 'cumplido', 'cancelado')),
    position         int         NOT NULL DEFAULT 0,
    version          int         NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, project_id),
    FOREIGN KEY (phase_id, project_id) REFERENCES project_phases (id, project_id),
    CHECK ((status = 'cumplido') = (completed_on IS NOT NULL))
);
CREATE INDEX milestones_project_id_idx ON milestones (project_id);

CREATE TABLE milestone_work_items (
    milestone_id uuid        NOT NULL,
    work_item_id uuid        NOT NULL,
    project_id   uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (milestone_id, work_item_id),
    FOREIGN KEY (milestone_id, project_id) REFERENCES milestones (id, project_id) ON DELETE CASCADE,
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items (id, project_id) ON DELETE CASCADE
);
CREATE INDEX milestone_work_items_work_item_id_idx ON milestone_work_items (work_item_id);

-- Objetivos -----------------------------------------------------------------

CREATE TABLE objectives (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title            text        NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 255),
    description      jsonb,
    description_text text        NOT NULL DEFAULT '',
    owner_id         uuid        NOT NULL REFERENCES users (id),
    scope            text        NOT NULL CHECK (scope IN ('organizacion', 'equipo', 'proyecto')),
    scope_team_id    uuid        REFERENCES teams (id),
    scope_project_id uuid        REFERENCES projects (id),
    period_start     date        NOT NULL,
    period_end       date        NOT NULL,
    status           text        NOT NULL DEFAULT 'borrador' CHECK (status IN ('borrador', 'activo', 'cerrado', 'cancelado')),
    version          int         NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (period_end >= period_start),
    CHECK (
        (scope = 'organizacion' AND scope_team_id IS NULL AND scope_project_id IS NULL) OR
        (scope = 'equipo' AND scope_team_id IS NOT NULL AND scope_project_id IS NULL) OR
        (scope = 'proyecto' AND scope_project_id IS NOT NULL AND scope_team_id IS NULL)
    )
);
CREATE INDEX objectives_scope_team_id_idx ON objectives (scope_team_id);
CREATE INDEX objectives_scope_project_id_idx ON objectives (scope_project_id);

CREATE TABLE key_results (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    objective_id  uuid           NOT NULL REFERENCES objectives (id),
    title         text           NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 255),
    unit          text           NOT NULL DEFAULT '',
    start_value   numeric(18, 4) NOT NULL,
    target_value  numeric(18, 4) NOT NULL,
    current_value numeric(18, 4) NOT NULL,
    calc_mode     text           NOT NULL CHECK (calc_mode IN ('manual', 'derivado')),
    position      int            NOT NULL DEFAULT 0,
    version       int            NOT NULL DEFAULT 1,
    created_at    timestamptz    NOT NULL DEFAULT now(),
    updated_at    timestamptz    NOT NULL DEFAULT now(),
    UNIQUE (id, objective_id),
    CHECK (start_value <> target_value)
);
CREATE INDEX key_results_objective_id_idx ON key_results (objective_id);

CREATE TABLE objective_projects (
    objective_id uuid NOT NULL REFERENCES objectives (id),
    project_id   uuid NOT NULL REFERENCES projects (id),
    PRIMARY KEY (objective_id, project_id)
);
CREATE INDEX objective_projects_project_id_idx ON objective_projects (project_id);

CREATE TABLE objective_milestones (
    objective_id uuid NOT NULL REFERENCES objectives (id),
    milestone_id uuid NOT NULL REFERENCES milestones (id) ON DELETE CASCADE,
    PRIMARY KEY (objective_id, milestone_id)
);
CREATE INDEX objective_milestones_milestone_id_idx ON objective_milestones (milestone_id);

-- A key result can only use milestones of its own objective (RF-108); unlinking
-- the milestone from the objective unlinks it from its key results (RF-109).
CREATE TABLE key_result_milestones (
    key_result_id uuid NOT NULL,
    objective_id  uuid NOT NULL,
    milestone_id  uuid NOT NULL,
    PRIMARY KEY (key_result_id, milestone_id),
    FOREIGN KEY (objective_id, milestone_id) REFERENCES objective_milestones (objective_id, milestone_id) ON DELETE CASCADE,
    FOREIGN KEY (key_result_id, objective_id) REFERENCES key_results (id, objective_id) ON DELETE CASCADE
);

CREATE TABLE key_result_work_items (
    key_result_id uuid NOT NULL REFERENCES key_results (id) ON DELETE CASCADE,
    work_item_id  uuid NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    PRIMARY KEY (key_result_id, work_item_id)
);
CREATE INDEX key_result_work_items_work_item_id_idx ON key_result_work_items (work_item_id);

-- Eventos de dominio (outbox) -----------------------------------------------

-- No foreign keys: an event outlives the work item it talks about.
CREATE TABLE domain_events (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id     uuid        NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    type         text        NOT NULL,
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    actor_id     uuid,
    project_id   uuid,
    subject_type text        NOT NULL,
    subject_id   uuid        NOT NULL,
    payload      jsonb       NOT NULL DEFAULT '{}'
);

CREATE TABLE event_consumers (
    name          text PRIMARY KEY,
    last_event_id bigint      NOT NULL DEFAULT 0,
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Agregados de work items ---------------------------------------------------

-- +goose StatementBegin
-- Subtree aggregates of one work item from its own values and its direct
-- children, which must be up to date. Returns the parent, to walk upwards.
CREATE FUNCTION work_item_refresh_one(p_id uuid) RETURNS uuid AS $$
    UPDATE work_items w SET
        (agg_estimated, agg_remaining, agg_count, agg_ratio_sum, agg_est_ratio_sum) = (
            SELECT coalesce(w.estimated_hours, 0) + coalesce(sum(k.agg_estimated), 0),
                   coalesce(w.remaining_hours, 0) + coalesce(sum(k.agg_remaining), 0),
                   1 + coalesce(sum(k.agg_count), 0)::int,
                   r.ratio + coalesce(sum(k.agg_ratio_sum), 0)::bigint,
                   coalesce(w.estimated_hours, 0) * r.ratio + coalesce(sum(k.agg_est_ratio_sum), 0)
            FROM (SELECT CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END AS ratio
                  FROM statuses s WHERE s.id = w.status_id) r
            LEFT JOIN work_items k ON k.parent_id = w.id
            GROUP BY r.ratio
        )
    WHERE w.id = p_id
    RETURNING w.parent_id;
$$ LANGUAGE sql;
-- +goose StatementEnd

-- +goose StatementBegin
-- Recomputes a work item and every ancestor, bottom-up (at most 10 updates).
CREATE FUNCTION work_item_refresh_up(p_id uuid) RETURNS void AS $$
DECLARE
    cur uuid := p_id;
BEGIN
    WHILE cur IS NOT NULL LOOP
        cur := work_item_refresh_one(cur);
    END LOOP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
-- Rebuilds every aggregate of a project from scratch, deepest level first.
CREATE FUNCTION work_item_recompute_project(p_project uuid) RETURNS void AS $$
DECLARE
    d int;
BEGIN
    FOR d IN REVERSE 9..0 LOOP
        PERFORM work_item_refresh_one(id) FROM work_items WHERE project_id = p_project AND depth = d;
    END LOOP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Datos iniciales (RF-11) -----------------------------------------------------

INSERT INTO statuses (name, name_normalized, color, is_closed, is_default, position) VALUES
    ('Nuevo',       'nuevo',       '#3b82f6', false, true,  1),
    ('En progreso', 'en progreso', '#f59e0b', false, false, 2),
    ('En revisión', 'en revisión', '#8b5cf6', false, false, 3),
    ('Resuelto',    'resuelto',    '#10b981', true,  false, 4),
    ('Cerrado',     'cerrado',     '#64748b', true,  false, 5),
    ('Rechazado',   'rechazado',   '#ef4444', true,  false, 6);

INSERT INTO work_item_types (name, name_normalized, color, position) VALUES
    ('Épica',        'épica',        '#a855f7', 1),
    ('Actividad',    'actividad',    '#3b82f6', 2),
    ('Subactividad', 'subactividad', '#06b6d4', 3),
    ('Story',        'story',        '#22c55e', 4),
    ('Tarea',        'tarea',        '#64748b', 5),
    ('Bug',          'bug',          '#ef4444', 6);

INSERT INTO priorities (name, name_normalized, color, is_default, position) VALUES
    ('Baja',    'baja',    '#94a3b8', false, 1),
    ('Normal',  'normal',  '#3b82f6', true,  2),
    ('Alta',    'alta',    '#f59e0b', false, 3),
    ('Urgente', 'urgente', '#ef4444', false, 4);

WITH m AS (
    INSERT INTO methodologies (name, name_normalized, position) VALUES ('Scrum', 'scrum', 1) RETURNING id
)
INSERT INTO phase_definitions (methodology_id, name, position, has_start_gate, has_finish_gate)
SELECT m.id, p.name, p.position, false, false
FROM m, (VALUES ('Preparación', 1), ('Ejecución por sprints', 2)) AS p (name, position);

WITH m AS (
    INSERT INTO methodologies (name, name_normalized, position) VALUES ('PRINCE2', 'prince2', 2) RETURNING id
)
INSERT INTO phase_definitions (methodology_id, name, position, has_start_gate, has_finish_gate)
SELECT m.id, p.name, p.position, true, true
FROM m, (VALUES ('Puesta en marcha', 1), ('Inicio', 2), ('Etapas de entrega', 3), ('Cierre', 4)) AS p (name, position);

-- +goose Down
DROP FUNCTION work_item_recompute_project(uuid);
DROP FUNCTION work_item_refresh_up(uuid);
DROP FUNCTION work_item_refresh_one(uuid);
DROP TABLE event_consumers;
DROP TABLE domain_events;
DROP TABLE key_result_work_items;
DROP TABLE key_result_milestones;
DROP TABLE objective_milestones;
DROP TABLE objective_projects;
DROP TABLE key_results;
DROP TABLE objectives;
DROP TABLE milestone_work_items;
DROP TABLE milestones;
DROP TABLE work_item_dependencies;
DROP TABLE work_item_custom_values;
DROP TABLE work_items;
DROP TABLE project_phases;
DROP TABLE project_custom_values;
DROP TABLE project_custom_fields;
DROP TABLE project_work_item_types;
DROP TABLE projects;
DROP TABLE custom_field_work_item_types;
DROP TABLE custom_fields;
DROP TABLE workflow_transitions;
DROP TABLE priorities;
DROP TABLE statuses;
DROP TABLE work_item_types;
DROP TABLE phase_definitions;
DROP TABLE methodologies;
