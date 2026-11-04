-- +goose Up
-- Spec 014: registro inmutable de puntos (RF-1 a RF-17), tabla de puntos por
-- talla (RF-24 a RF-28) y niveles (RF-29 a RF-36). Solo aditiva.

CREATE TABLE point_ledger (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    seq               bigint      NOT NULL GENERATED ALWAYS AS IDENTITY UNIQUE,
    user_id           uuid        NOT NULL REFERENCES users (id),
    xp                int         NOT NULL,
    coins             int         NOT NULL,
    reason            text        NOT NULL CHECK (reason IN ('close', 'reopen', 'adjust')),
    work_item_id      uuid        REFERENCES work_items (id) ON DELETE SET NULL,
    event_id          uuid,
    reverses_event_id uuid,
    note              text,
    actor_id          uuid        REFERENCES users (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, user_id),
    CHECK (xp <> 0 OR coins <> 0)
);
CREATE INDEX point_ledger_user_seq_idx ON point_ledger (user_id, seq);
CREATE INDEX point_ledger_work_item_idx ON point_ledger (work_item_id);
CREATE INDEX point_ledger_created_idx ON point_ledger (created_at);

-- A movement is never changed nor removed (RF-2). Removing a work item without
-- children (spec 004, RF-85) only clears the link, which the FK does.
-- +goose StatementBegin
CREATE FUNCTION point_ledger_immutable() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.work_item_id IS NULL AND OLD.work_item_id IS NOT NULL
       AND (NEW.id, NEW.seq, NEW.user_id, NEW.xp, NEW.coins, NEW.reason, NEW.event_id, NEW.reverses_event_id, NEW.note, NEW.actor_id, NEW.created_at)
           IS NOT DISTINCT FROM
           (OLD.id, OLD.seq, OLD.user_id, OLD.xp, OLD.coins, OLD.reason, OLD.event_id, OLD.reverses_event_id, OLD.note, OLD.actor_id, OLD.created_at) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'point_ledger es inmutable';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER point_ledger_immutable BEFORE UPDATE OR DELETE ON point_ledger
    FOR EACH ROW EXECUTE FUNCTION point_ledger_immutable();

-- A size without a row is worth 0 XP and 0 coins (RF-25); the row with a null
-- size is «sin talla».
CREATE TABLE point_values (
    size_id    uuid        REFERENCES work_item_sizes (id),
    xp         int         NOT NULL CHECK (xp BETWEEN 0 AND 1000),
    coins      int         NOT NULL CHECK (coins BETWEEN 0 AND 1000),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX point_values_size_idx ON point_values (size_id);
CREATE UNIQUE INDEX point_values_no_size_idx ON point_values ((true)) WHERE size_id IS NULL;

INSERT INTO point_values (size_id, xp, coins) VALUES (NULL, 5, 1);
INSERT INTO point_values (size_id, xp, coins)
SELECT s.id, v.xp, v.coins
FROM (VALUES ('xs', 5, 1), ('s', 10, 2), ('m', 20, 4), ('l', 40, 8), ('xl', 80, 16)) AS v (name, xp, coins)
JOIN work_item_sizes s ON s.name_normalized = v.name AND s.retired_at IS NULL;

CREATE TABLE levels (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
    min_xp     int         NOT NULL UNIQUE CHECK (min_xp >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO levels (name, min_xp)
SELECT 'Nivel ' || n, 50 * n * (n - 1) FROM generate_series(1, 10) AS n;

-- Everybody starts at 0 (RF-53): the consumer begins after the last event.
INSERT INTO event_consumers (name, last_event_id)
VALUES ('gamification', COALESCE((SELECT max(id) FROM domain_events), 0))
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DELETE FROM event_consumers WHERE name = 'gamification';
DROP TABLE levels;
DROP TABLE point_values;
DROP TABLE point_ledger;
DROP FUNCTION point_ledger_immutable();
