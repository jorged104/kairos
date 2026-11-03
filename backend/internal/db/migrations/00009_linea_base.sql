-- +goose Up
-- Spec 009: línea base por proyecto (RF-1 a RF-12) y configuración del
-- semáforo (RF-32, RF-36 a RF-41). Solo aditiva.

CREATE TABLE project_baselines (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    version    int         NOT NULL CHECK (version > 0),
    reason     text        CHECK (char_length(reason) BETWEEN 1 AND 500),
    created_by uuid        NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, version)
);

-- Deleting a work item takes it out of every baseline (Casos límite).
CREATE TABLE project_baseline_items (
    baseline_id  uuid NOT NULL REFERENCES project_baselines (id) ON DELETE CASCADE,
    work_item_id uuid NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    start_date   date,
    due_date     date,
    PRIMARY KEY (baseline_id, work_item_id)
);
CREATE INDEX project_baseline_items_work_item_idx ON project_baseline_items (work_item_id);

-- A single row. A null name is the default one of the translations (RF-39).
CREATE TABLE schedule_settings (
    id             boolean      PRIMARY KEY DEFAULT true CHECK (id),
    upper          numeric(4,1) NOT NULL DEFAULT 5,
    risk           numeric(4,1) NOT NULL DEFAULT -5,
    late           numeric(4,1) NOT NULL DEFAULT -15,
    name_ahead     text CHECK (char_length(name_ahead) BETWEEN 1 AND 30),
    name_on_track  text CHECK (char_length(name_on_track) BETWEEN 1 AND 30),
    name_at_risk   text CHECK (char_length(name_at_risk) BETWEEN 1 AND 30),
    name_late      text CHECK (char_length(name_late) BETWEEN 1 AND 30),
    color_ahead    text         NOT NULL DEFAULT 'blue',
    color_on_track text         NOT NULL DEFAULT 'green',
    color_at_risk  text         NOT NULL DEFAULT 'amber',
    color_late     text         NOT NULL DEFAULT 'red',
    version        int          NOT NULL DEFAULT 1,
    updated_at     timestamptz  NOT NULL DEFAULT now(),
    CHECK (upper > risk AND risk > late),
    CHECK (upper BETWEEN -100 AND 100 AND risk BETWEEN -100 AND 100 AND late BETWEEN -100 AND 100)
);
INSERT INTO schedule_settings DEFAULT VALUES;

-- +goose Down
DROP TABLE schedule_settings;
DROP TABLE project_baseline_items;
DROP TABLE project_baselines;
