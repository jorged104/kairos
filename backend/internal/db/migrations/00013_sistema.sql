-- +goose Up
-- Spec 015: configuración del sistema editable desde la interfaz (RF-1 a
-- RF-47) y modo claro u oscuro de cada usuario (RF-48 a RF-51). Solo aditiva:
-- la fila de system_settings la crea la API al arrancar con las variables de
-- entorno (RF-8), y la versión anterior de la API no la lee.

CREATE TABLE system_settings (
    id                   boolean     PRIMARY KEY DEFAULT true CHECK (id),
    version              bigint      NOT NULL DEFAULT 1,
    auth_mode            text        NOT NULL CHECK (auth_mode IN ('local', 'oidc', 'both')),
    registration_enabled boolean     NOT NULL DEFAULT false,
    default_role         text        NOT NULL DEFAULT 'member' CHECK (default_role IN ('', 'viewer', 'member', 'lead')),
    oidc_issuer          text        NOT NULL DEFAULT '',
    oidc_discovery_url   text        NOT NULL DEFAULT '',
    oidc_client_id       text        NOT NULL DEFAULT '',
    oidc_client_secret   bytea,
    oidc_test            jsonb,
    files_kind           text        NOT NULL DEFAULT 'db' CHECK (files_kind IN ('db', 's3')),
    s3_endpoint          text        NOT NULL DEFAULT '',
    s3_bucket            text        NOT NULL DEFAULT '',
    s3_region            text        NOT NULL DEFAULT '',
    s3_access_key        text        NOT NULL DEFAULT '',
    s3_secret_key        bytea,
    s3_use_ssl           boolean     NOT NULL DEFAULT true,
    theme_palette        text        NOT NULL DEFAULT 'neutral',
    theme_primary        text        NOT NULL DEFAULT 'default',
    theme_radius         text        NOT NULL DEFAULT 'soft',
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_preferences (
    user_id    uuid        PRIMARY KEY REFERENCES users (id),
    theme_mode text        NOT NULL DEFAULT 'system' CHECK (theme_mode IN ('light', 'dark', 'system')),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- A login state is a real login or the administrator's test (RF-28 to RF-30).
ALTER TABLE oidc_login_states ADD COLUMN purpose text NOT NULL DEFAULT 'login' CHECK (purpose IN ('login', 'test'));

-- +goose Down
ALTER TABLE oidc_login_states DROP COLUMN purpose;
DROP TABLE user_preferences;
DROP TABLE system_settings;
