-- +goose Up
-- Spec 023: el rol Dirección. Solo amplía los valores admitidos: la versión
-- anterior de la API no asigna ni lee este rol.
ALTER TABLE user_roles DROP CONSTRAINT user_roles_role_check;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_role_check
    CHECK (role IN ('admin', 'director', 'lead', 'member', 'viewer'));

-- +goose Down
DELETE FROM user_roles WHERE role = 'director';
ALTER TABLE user_roles DROP CONSTRAINT user_roles_role_check;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_role_check
    CHECK (role IN ('admin', 'lead', 'member', 'viewer'));
