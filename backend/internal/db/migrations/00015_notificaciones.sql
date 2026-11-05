-- +goose Up
-- Spec 017: notificaciones de comentarios, menciones y cambios de las tareas,
-- con seguidores por tarea. Solo aditiva: la versión anterior de la API no lee
-- nada de esto; el trigger de seguidores también funciona con ella.

-- Who follows a work item. following = false is an explicit «stop following»
-- (RF-4); no row is not following.
CREATE TABLE work_item_follows (
    work_item_id uuid        NOT NULL REFERENCES work_items (id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users (id),
    following    boolean     NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (work_item_id, user_id)
);
CREATE INDEX work_item_follows_user_idx ON work_item_follows (user_id);

-- An assignee follows the work item (RF-1). Rewriting the assignees never
-- undoes an explicit «stop following»; the API re-follows the new ones (RF-5).
-- +goose StatementBegin
CREATE FUNCTION work_item_assignee_follows() RETURNS trigger AS $$
BEGIN
    INSERT INTO work_item_follows (work_item_id, user_id, following)
    VALUES (NEW.work_item_id, NEW.user_id, true)
    ON CONFLICT (work_item_id, user_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER work_item_assignees_follow AFTER INSERT ON work_item_assignees
    FOR EACH ROW EXECUTE FUNCTION work_item_assignee_follows();

INSERT INTO work_item_follows (work_item_id, user_id, following)
SELECT work_item_id, user_id, true FROM work_item_assignees
ON CONFLICT DO NOTHING;

-- One notification per person and element while it is unread: the events of
-- that element join it (RF-21). actors holds the most recent first.
CREATE TABLE notifications (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users (id),
    subject_type text        NOT NULL CHECK (subject_type IN ('work_item', 'milestone', 'project')),
    subject_id   uuid        NOT NULL,
    project_id   uuid        NOT NULL,
    actors       uuid[]      NOT NULL,
    events       integer     NOT NULL DEFAULT 1,
    mention      boolean     NOT NULL DEFAULT false,
    last_kind    text        NOT NULL,
    last_data    jsonb       NOT NULL DEFAULT '{}',
    via          jsonb,
    read_at      timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL
);
CREATE UNIQUE INDEX notifications_unread_subject_idx ON notifications (user_id, subject_type, subject_id) WHERE read_at IS NULL;
CREATE INDEX notifications_user_idx ON notifications (user_id, updated_at DESC);
CREATE INDEX notifications_updated_idx ON notifications (updated_at);

-- Events already turned into notifications, so a repeated delivery counts once.
CREATE TABLE notified_events (
    event_id   uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, user_id)
);
CREATE INDEX notified_events_created_idx ON notified_events (created_at);

-- No notifications for what happened before (fuera de alcance).
INSERT INTO event_consumers (name, last_event_id)
VALUES ('notifications', COALESCE((SELECT max(id) FROM domain_events), 0))
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM event_consumers WHERE name = 'notifications';
DROP TABLE notified_events;
DROP TABLE notifications;
DROP TRIGGER work_item_assignees_follow ON work_item_assignees;
DROP FUNCTION work_item_assignee_follows();
DROP TABLE work_item_follows;
