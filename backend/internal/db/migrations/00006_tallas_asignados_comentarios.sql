-- +goose Up
-- Spec 006: tallas del catálogo (RF-1 a RF-11), varios asignados por work item
-- (RF-12 a RF-21) y comentarios (RF-22 a RF-55). Solo aditiva: assignee_id se
-- queda y la versión anterior de la API lo sigue leyendo y escribiendo.

CREATE TABLE work_item_sizes (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    weight          smallint    NOT NULL DEFAULT 1 CHECK (weight BETWEEN 0 AND 100),
    position        int         NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX work_item_sizes_active_name_idx ON work_item_sizes (name_normalized) WHERE retired_at IS NULL;

INSERT INTO work_item_sizes (name, name_normalized, weight, position) VALUES
    ('XS', 'xs', 1, 1), ('S', 's', 2, 2), ('M', 'm', 3, 3), ('L', 'l', 5, 4), ('XL', 'xl', 8, 5);

ALTER TABLE work_items ADD COLUMN size_id uuid REFERENCES work_item_sizes (id);

-- Everyone working on a work item, all with the same role (RF-12).
CREATE TABLE work_item_assignees (
    work_item_id uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id),
    position     int         NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (work_item_id, user_id)
);
CREATE INDEX work_item_assignees_user_idx ON work_item_assignees (user_id);

INSERT INTO work_item_assignees (work_item_id, user_id)
SELECT id, assignee_id FROM work_items WHERE assignee_id IS NOT NULL;

-- +goose StatementBegin
-- While the previous API runs it only writes assignee_id: its change replaces
-- the list. The new API writes the list first and then assignee_id with its
-- first person, already in the list, so nothing is replaced.
CREATE FUNCTION work_item_assignee_sync() RETURNS trigger AS $$
BEGIN
    IF NEW.assignee_id IS NULL THEN
        IF TG_OP = 'UPDATE' AND OLD.assignee_id IS NOT NULL THEN
            DELETE FROM work_item_assignees WHERE work_item_id = NEW.id;
        END IF;
    ELSIF NOT EXISTS (SELECT 1 FROM work_item_assignees WHERE work_item_id = NEW.id AND user_id = NEW.assignee_id) THEN
        DELETE FROM work_item_assignees WHERE work_item_id = NEW.id;
        INSERT INTO work_item_assignees (work_item_id, user_id) VALUES (NEW.id, NEW.assignee_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER work_items_assignee_sync AFTER INSERT OR UPDATE OF assignee_id ON work_items
    FOR EACH ROW EXECUTE FUNCTION work_item_assignee_sync();

-- Comments: a root (thread_id NULL) and its replies, one level (RF-28). The
-- root keeps the thread's state: resolved (RF-36) and the anchor in the page
-- (RF-45, RF-48). They go with their work item (RF-40).
CREATE TABLE work_item_comments (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    work_item_id uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    thread_id    uuid        REFERENCES work_item_comments (id) ON DELETE CASCADE,
    author_id    uuid        NOT NULL REFERENCES users (id),
    body         jsonb,
    body_text    text        NOT NULL DEFAULT '',
    quote        text,
    anchored     boolean     NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    edited_at    timestamptz,
    deleted_at   timestamptz,
    resolved_at  timestamptz,
    resolved_by  uuid        REFERENCES users (id),
    CHECK (thread_id IS NULL OR (NOT anchored AND resolved_at IS NULL AND quote IS NULL)),
    CHECK ((resolved_at IS NULL) = (resolved_by IS NULL))
);
CREATE INDEX work_item_comments_item_idx ON work_item_comments (work_item_id, created_at);
CREATE INDEX work_item_comments_thread_idx ON work_item_comments (thread_id, created_at);

-- The files of the comments of a work item are owned by (work_item_comment, work item).
ALTER TABLE page_files DROP CONSTRAINT page_files_owner_kind_check;
ALTER TABLE page_files ADD CONSTRAINT page_files_owner_kind_check
    CHECK (owner_kind IN ('project', 'milestone', 'work_item', 'work_item_comment'));

-- +goose Down
DELETE FROM page_files WHERE owner_kind = 'work_item_comment';
ALTER TABLE page_files DROP CONSTRAINT page_files_owner_kind_check;
ALTER TABLE page_files ADD CONSTRAINT page_files_owner_kind_check
    CHECK (owner_kind IN ('project', 'milestone', 'work_item'));
DROP TABLE work_item_comments;
DROP TRIGGER work_items_assignee_sync ON work_items;
DROP FUNCTION work_item_assignee_sync();
DROP TABLE work_item_assignees;
ALTER TABLE work_items DROP COLUMN size_id;
DROP TABLE work_item_sizes;
