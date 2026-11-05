-- +goose Up
-- Spec 016: conexiones de clientes de IA por MCP (RF-1 a RF-26) y la marca
-- «vía IA» de lo que cambian (RF-42 a RF-44). Solo aditiva: la versión
-- anterior de la API no lee nada de esto.

-- Off on a new installation (RF-1); the administrator turns it on (RF-2).
ALTER TABLE system_settings ADD COLUMN mcp_enabled boolean NOT NULL DEFAULT false;

-- A connection is reached by its secret URL; only the hash of the secret is
-- kept, so the URL cannot be rebuilt from the database (RF-13).
CREATE TABLE mcp_connections (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid        NOT NULL REFERENCES users (id),
    name         text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    secret_hash  bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    last_used_at timestamptz,
    last_client  text,
    revoked_at   timestamptz,
    revoked_by   uuid        REFERENCES users (id),
    CHECK ((revoked_at IS NULL) = (revoked_by IS NULL))
);
CREATE INDEX mcp_connections_user_idx ON mcp_connections (user_id, created_at);

-- Which connection made a change, null for the interface (RF-43, RF-44).
ALTER TABLE audit_log ADD COLUMN via jsonb;
ALTER TABLE work_item_comments ADD COLUMN via jsonb;

-- +goose Down
ALTER TABLE work_item_comments DROP COLUMN via;
ALTER TABLE audit_log DROP COLUMN via;
DROP TABLE mcp_connections;
ALTER TABLE system_settings DROP COLUMN mcp_enabled;
