-- +goose Up
-- Spec 020: informes de proyecto con IA. Solo aditiva: la versión anterior
-- de la API no lee nada de esto.

-- The AI service compatible with OpenAI's API (RF-1 to RF-10). The key is
-- sealed like the other secrets of spec 015 (RF-7).
ALTER TABLE system_settings
    ADD COLUMN ai_enabled  boolean NOT NULL DEFAULT false,
    ADD COLUMN ai_base_url text    NOT NULL DEFAULT '' CHECK (length(ai_base_url) <= 500),
    ADD COLUMN ai_api_key  bytea,
    ADD COLUMN ai_model    text    NOT NULL DEFAULT '' CHECK (length(ai_model) <= 200);

-- The kinds of report the administrator defines, each with its prompt
-- (RF-18 to RF-27).
CREATE TABLE report_templates (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    name_normalized text        NOT NULL,
    description     text        NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    prompt          text        NOT NULL CHECK (length(prompt) BETWEEN 1 AND 20000),
    archived_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
-- Unique among the active ones (RF-21).
CREATE UNIQUE INDEX report_templates_name_idx ON report_templates (name_normalized) WHERE archived_at IS NULL;

INSERT INTO report_templates (name, name_normalized, description, prompt) VALUES (
    'Estado del proyecto', 'estado del proyecto',
    'Situación del proyecto y de sus subproyectos a la fecha de hoy.',
    'Escribe un informe del estado del proyecto para su equipo y para quien lo patrocina.

Incluye, por este orden:
1. Resumen: tres o cuatro frases con la situación general y el avance real frente al plan.
2. Avance: avance real, avance planificado y brecha del proyecto y, si los hay, de cada subproyecto, en una tabla.
3. Hitos: los terminados recientemente, los próximos y los atrasados, con sus fechas.
4. Tareas atrasadas: las más relevantes, con su responsable y cuántos días llevan de retraso.
5. Próximas entregas: lo que vence en las próximas dos semanas.
6. Riesgos y bloqueos: los que se deduzcan de los datos, sin inventar.

Sé concreto, usa cifras de los datos y no repitas información.'
);

-- The reports of a project (RF-28 to RF-70): only the last version is kept
-- (RF-63); the name of the template is copied so archiving it changes
-- nothing (RF-26).
CREATE TABLE project_reports (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    template_id   uuid        REFERENCES report_templates (id) ON DELETE SET NULL,
    template_name text        NOT NULL DEFAULT '',
    title         text        NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    content       text        NOT NULL CHECK (length(content) BETWEEN 1 AND 200000),
    author_id     uuid        NOT NULL REFERENCES users (id),
    updated_by    uuid        NOT NULL REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    version       integer     NOT NULL DEFAULT 1
);
CREATE INDEX project_reports_project_idx ON project_reports (project_id, updated_at DESC);

-- +goose Down
DROP TABLE project_reports;
DROP TABLE report_templates;
ALTER TABLE system_settings
    DROP COLUMN ai_enabled, DROP COLUMN ai_base_url, DROP COLUMN ai_api_key, DROP COLUMN ai_model;
