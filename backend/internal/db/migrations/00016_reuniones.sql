-- +goose Up
-- Spec 018: reuniones con notas, plantillas, comentarios y tareas. Solo
-- aditiva: la versión anterior de la API no lee reuniones, y los comentarios
-- que lee (los de tareas) siguen teniendo su tarea.

-- A meeting belongs to an owner; today only a project (RF-2). project_id is
-- the project of reference, for permissions and files, whatever the owner.
CREATE TABLE meetings (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type       text        NOT NULL CHECK (owner_type IN ('project')),
    owner_id         uuid        NOT NULL,
    project_id       uuid        NOT NULL REFERENCES projects (id),
    title            text        NOT NULL CHECK (length(title) BETWEEN 1 AND 255),
    starts_at        timestamptz NOT NULL,
    location         text        NOT NULL DEFAULT '' CHECK (length(location) <= 500),
    description      jsonb,
    description_text text        NOT NULL DEFAULT '',
    page_version     integer     NOT NULL DEFAULT 1,
    version          integer     NOT NULL DEFAULT 1,
    template_id      uuid,
    created_by       uuid        NOT NULL REFERENCES users (id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meetings_project_idx ON meetings (project_id, starts_at);
CREATE INDEX meetings_owner_idx ON meetings (owner_type, owner_id, starts_at);
-- Its notes are a page like the others (spec 005): a change of the
-- description moves page_version.
CREATE TRIGGER meetings_page_version BEFORE UPDATE ON meetings
    FOR EACH ROW EXECUTE FUNCTION bump_page_version();

-- Global templates (project_id NULL) and templates of a project (RF-29 to RF-31).
CREATE TABLE meeting_templates (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid        REFERENCES projects (id) ON DELETE CASCADE,
    name            text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    name_normalized text        NOT NULL,
    title           text        NOT NULL DEFAULT '' CHECK (length(title) <= 255),
    location        text        NOT NULL DEFAULT '' CHECK (length(location) <= 500),
    page            jsonb,
    position        integer     NOT NULL DEFAULT 0,
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
-- Unique in its scope (RF-33).
CREATE UNIQUE INDEX meeting_templates_name_idx
    ON meeting_templates (coalesce(project_id, '00000000-0000-0000-0000-000000000000'::uuid), name_normalized);

-- The meeting a work item came from (RF-26); deleting it keeps the work item (RF-13).
ALTER TABLE work_items ADD COLUMN meeting_id uuid REFERENCES meetings (id) ON DELETE SET NULL;
CREATE INDEX work_items_meeting_idx ON work_items (meeting_id) WHERE meeting_id IS NOT NULL;

-- Comments are on a work item or on a meeting, never both (RF-20).
ALTER TABLE work_item_comments ADD COLUMN meeting_id uuid REFERENCES meetings (id) ON DELETE CASCADE;
ALTER TABLE work_item_comments ALTER COLUMN work_item_id DROP NOT NULL;
ALTER TABLE work_item_comments ADD CONSTRAINT work_item_comments_target_check
    CHECK (num_nonnulls(work_item_id, meeting_id) = 1);
CREATE INDEX work_item_comments_meeting_idx ON work_item_comments (meeting_id, created_at) WHERE meeting_id IS NOT NULL;

ALTER TABLE page_files DROP CONSTRAINT page_files_owner_kind_check;
ALTER TABLE page_files ADD CONSTRAINT page_files_owner_kind_check
    CHECK (owner_kind IN ('project', 'milestone', 'work_item', 'work_item_comment', 'meeting', 'meeting_comment'));

ALTER TABLE notifications DROP CONSTRAINT notifications_subject_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_subject_type_check
    CHECK (subject_type IN ('work_item', 'milestone', 'project', 'meeting'));

-- +goose Down
DELETE FROM notifications WHERE subject_type = 'meeting';
ALTER TABLE notifications DROP CONSTRAINT notifications_subject_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_subject_type_check
    CHECK (subject_type IN ('work_item', 'milestone', 'project'));
DELETE FROM page_files WHERE owner_kind IN ('meeting', 'meeting_comment');
ALTER TABLE page_files DROP CONSTRAINT page_files_owner_kind_check;
ALTER TABLE page_files ADD CONSTRAINT page_files_owner_kind_check
    CHECK (owner_kind IN ('project', 'milestone', 'work_item', 'work_item_comment'));
DELETE FROM work_item_comments WHERE meeting_id IS NOT NULL;
ALTER TABLE work_item_comments DROP CONSTRAINT work_item_comments_target_check;
ALTER TABLE work_item_comments ALTER COLUMN work_item_id SET NOT NULL;
ALTER TABLE work_item_comments DROP COLUMN meeting_id;
ALTER TABLE work_items DROP COLUMN meeting_id;
DROP TABLE meeting_templates;
DROP TABLE meetings;
