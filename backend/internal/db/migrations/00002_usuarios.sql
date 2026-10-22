-- +goose Up
-- Spec 003: usuarios, identidades, roles, equipos, atributos, homologaciones y auditoría.
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text        NOT NULL,
    email            text        NOT NULL,
    email_normalized text        NOT NULL UNIQUE,
    status           text        NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'inactive', 'offboarded')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_identities (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id),
    issuer     text        NOT NULL,
    subject    text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);
CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);

CREATE TABLE local_credentials (
    user_id       uuid PRIMARY KEY REFERENCES users (id),
    password_hash text        NOT NULL,
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_roles (
    user_id    uuid        NOT NULL REFERENCES users (id),
    role       text        NOT NULL CHECK (role IN ('admin', 'lead', 'member', 'viewer')),
    source     text        NOT NULL CHECK (source IN ('manual', 'idp')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

CREATE TABLE teams (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL UNIQUE,
    parent_id       uuid        REFERENCES teams (id),
    external_ref    text,
    archived_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX teams_parent_id_idx ON teams (parent_id);

CREATE TABLE team_members (
    team_id    uuid        NOT NULL REFERENCES teams (id),
    user_id    uuid        NOT NULL REFERENCES users (id),
    source     text        NOT NULL CHECK (source IN ('manual', 'idp')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX team_members_user_id_idx ON team_members (user_id);

CREATE TABLE team_leaders (
    team_id    uuid        NOT NULL REFERENCES teams (id),
    user_id    uuid        NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX team_leaders_user_id_idx ON team_leaders (user_id);

CREATE TABLE attributes (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text        NOT NULL,
    name_normalized text        NOT NULL,
    type            text        NOT NULL CHECK (type IN ('text', 'number', 'date', 'boolean', 'select')),
    required        boolean     NOT NULL DEFAULT false,
    options         jsonb       NOT NULL DEFAULT '[]',
    retired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX attributes_active_name_idx ON attributes (name_normalized) WHERE retired_at IS NULL;

CREATE TABLE user_attribute_values (
    user_id      uuid        NOT NULL REFERENCES users (id),
    attribute_id uuid        NOT NULL REFERENCES attributes (id),
    value        jsonb       NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, attribute_id)
);
CREATE INDEX user_attribute_values_attribute_id_idx ON user_attribute_values (attribute_id);

CREATE TABLE idp_mappings (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    issuer         text        NOT NULL,
    claim          text        NOT NULL CHECK (claim IN ('roles', 'groups')),
    external_value text        NOT NULL,
    target_type    text        NOT NULL CHECK (target_type IN ('role', 'team')),
    target         text        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, claim, external_value, target_type, target)
);

CREATE TABLE audit_log (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_id    uuid REFERENCES users (id),
    actor_kind  text        NOT NULL CHECK (actor_kind IN ('user', 'idp', 'system')),
    action      text        NOT NULL,
    target_type text        NOT NULL,
    target_id   text        NOT NULL,
    before      jsonb,
    after       jsonb
);
CREATE INDEX audit_log_occurred_at_idx ON audit_log (occurred_at);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id);
CREATE INDEX audit_log_actor_id_idx ON audit_log (actor_id);

-- +goose StatementBegin
CREATE FUNCTION audit_log_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log es de solo inserción';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

CREATE TABLE oidc_login_states (
    state         text PRIMARY KEY,
    nonce         text        NOT NULL,
    code_verifier text        NOT NULL,
    return_to     text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE oidc_login_states;
DROP TRIGGER audit_log_immutable ON audit_log;
DROP FUNCTION audit_log_immutable();
DROP TABLE audit_log;
DROP TABLE idp_mappings;
DROP TABLE user_attribute_values;
DROP TABLE attributes;
DROP TABLE team_leaders;
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE user_roles;
DROP TABLE local_credentials;
DROP TABLE user_identities;
DROP TABLE users;
