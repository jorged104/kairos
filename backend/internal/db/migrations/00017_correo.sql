-- +goose Up
-- Spec 019: correo saliente por SMTP, invitaciones y recuperación de
-- contraseña. Solo aditiva: la versión anterior de la API no lee nada de esto.

-- The outgoing mail, set in the system screen (RF-1 to RF-9). The password
-- is sealed like the other secrets of spec 015 (RF-4).
ALTER TABLE system_settings
    ADD COLUMN mail_enabled    boolean NOT NULL DEFAULT false,
    ADD COLUMN smtp_host       text    NOT NULL DEFAULT '',
    ADD COLUMN smtp_port       integer NOT NULL DEFAULT 587 CHECK (smtp_port BETWEEN 1 AND 65535),
    ADD COLUMN smtp_security   text    NOT NULL DEFAULT 'starttls' CHECK (smtp_security IN ('none', 'starttls', 'tls')),
    ADD COLUMN smtp_username   text    NOT NULL DEFAULT '',
    ADD COLUMN smtp_password   bytea,
    ADD COLUMN mail_from_email text    NOT NULL DEFAULT '',
    ADD COLUMN mail_from_name  text    NOT NULL DEFAULT 'Kairos';

-- Single-use links to create or reset a password. Only the hash of the
-- secret is kept, so a link cannot be rebuilt from the database (RF-37, RF-38).
CREATE TABLE user_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id),
    purpose    text        NOT NULL CHECK (purpose IN ('invite', 'reset')),
    token_hash bytea       NOT NULL UNIQUE,
    created_by uuid        REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    revoked_at timestamptz
);
CREATE INDEX user_tokens_user_idx ON user_tokens (user_id, purpose, created_at);

-- +goose Down
DROP TABLE user_tokens;
ALTER TABLE system_settings
    DROP COLUMN mail_enabled, DROP COLUMN smtp_host, DROP COLUMN smtp_port, DROP COLUMN smtp_security,
    DROP COLUMN smtp_username, DROP COLUMN smtp_password, DROP COLUMN mail_from_email, DROP COLUMN mail_from_name;
