-- +goose Up
-- Spec 005, páginas tipo Notion: versión propia de la página (RF-34, RF-59) y
-- archivos de las páginas (RF-14 a RF-24, RF-49 a RF-55). Solo aditiva: la
-- descripción sigue en la misma columna y la versión anterior de la API la
-- lee y la escribe igual.

ALTER TABLE projects   ADD COLUMN page_version int NOT NULL DEFAULT 1;
ALTER TABLE milestones ADD COLUMN page_version int NOT NULL DEFAULT 1;
ALTER TABLE work_items ADD COLUMN page_version int NOT NULL DEFAULT 1;

-- +goose StatementBegin
-- Any change of the description bumps the page version, also one made by the
-- previous API through the full PATCH, so the autosave sees it as a conflict.
CREATE FUNCTION bump_page_version() RETURNS trigger AS $$
BEGIN
    IF NEW.description IS DISTINCT FROM OLD.description THEN
        NEW.page_version := OLD.page_version + 1;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER projects_page_version BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION bump_page_version();
CREATE TRIGGER milestones_page_version BEFORE UPDATE ON milestones
    FOR EACH ROW EXECUTE FUNCTION bump_page_version();
CREATE TRIGGER work_items_page_version BEFORE UPDATE ON work_items
    FOR EACH ROW EXECUTE FUNCTION bump_page_version();

-- Images and attachments of a page. The owner is a project, a milestone or a
-- work item of project_id; the service checks it, there is no polymorphic FK.
-- Rows are never deleted here: deleted_at marks them (RF-49 to RF-52).
CREATE TABLE page_files (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid        NOT NULL REFERENCES projects (id),
    owner_kind   text        NOT NULL CHECK (owner_kind IN ('project', 'milestone', 'work_item')),
    owner_id     uuid        NOT NULL,
    kind         text        NOT NULL CHECK (kind IN ('image', 'file')),
    name         text        NOT NULL CHECK (length(name) BETWEEN 1 AND 255),
    content_type text        NOT NULL,
    size_bytes   bigint      NOT NULL CHECK (size_bytes BETWEEN 0 AND 10485760),
    storage      text        NOT NULL CHECK (storage IN ('db', 's3')),
    s3_key       text,
    created_by   uuid        NOT NULL REFERENCES users (id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    CHECK ((storage = 's3') = (s3_key IS NOT NULL))
);
CREATE INDEX page_files_owner ON page_files (owner_kind, owner_id) WHERE deleted_at IS NULL;

-- Content of the files kept in the database, apart so listings never read it.
CREATE TABLE page_file_data (
    file_id uuid  PRIMARY KEY REFERENCES page_files (id) ON DELETE CASCADE,
    data    bytea NOT NULL
);

-- +goose Down
DROP TABLE page_file_data;
DROP TABLE page_files;
DROP TRIGGER work_items_page_version ON work_items;
DROP TRIGGER milestones_page_version ON milestones;
DROP TRIGGER projects_page_version ON projects;
DROP FUNCTION bump_page_version();
ALTER TABLE work_items DROP COLUMN page_version;
ALTER TABLE milestones DROP COLUMN page_version;
ALTER TABLE projects DROP COLUMN page_version;
