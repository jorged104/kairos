-- +goose Up
-- Spec 007: etiquetas globales del catálogo (RF-36 a RF-46). Solo aditiva. Las
-- métricas y el historial de las tareas salen de audit_log y no necesitan esquema.

CREATE TABLE labels (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    color           text        NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9a-f]{6}$'),
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX labels_active_name_idx ON labels (name_normalized) WHERE retired_at IS NULL;

CREATE TABLE work_item_labels (
    work_item_id uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    label_id     uuid        NOT NULL REFERENCES labels (id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (work_item_id, label_id)
);
CREATE INDEX work_item_labels_label_idx ON work_item_labels (label_id);

-- +goose Down
DROP TABLE work_item_labels;
DROP TABLE labels;
