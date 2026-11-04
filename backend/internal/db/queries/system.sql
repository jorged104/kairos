-- Spec 015: system settings and user preferences.

-- name: GetSystemSettings :one
SELECT * FROM system_settings WHERE id;

-- name: LockSystemSettings :one
SELECT * FROM system_settings WHERE id FOR UPDATE;

-- name: SystemSettingsVersion :one
SELECT version FROM system_settings WHERE id;

-- name: InsertSystemSettings :exec
-- The first start copies the environment (RF-8); a concurrent start loses.
INSERT INTO system_settings (auth_mode, registration_enabled, default_role, oidc_issuer, oidc_discovery_url, oidc_client_id,
    oidc_client_secret, files_kind, s3_endpoint, s3_bucket, s3_region, s3_access_key, s3_secret_key, s3_use_ssl)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (id) DO NOTHING;

-- name: UpdateSystemSettings :one
UPDATE system_settings SET
    version = version + 1,
    auth_mode = $1, registration_enabled = $2, default_role = $3,
    oidc_issuer = $4, oidc_discovery_url = $5, oidc_client_id = $6, oidc_client_secret = $7, oidc_test = $8,
    files_kind = $9, s3_endpoint = $10, s3_bucket = $11, s3_region = $12, s3_access_key = $13, s3_secret_key = $14, s3_use_ssl = $15,
    theme_palette = $16, theme_primary = $17, theme_radius = $18, updated_at = now()
WHERE id
RETURNING *;

-- name: CountLiveFiles :one
SELECT count(*) FROM page_files WHERE deleted_at IS NULL;

-- name: GetThemeMode :one
SELECT theme_mode FROM user_preferences WHERE user_id = $1;

-- name: SetThemeMode :exec
INSERT INTO user_preferences (user_id, theme_mode) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET theme_mode = EXCLUDED.theme_mode, updated_at = now();

-- name: HasLocalCredential :one
SELECT EXISTS (SELECT 1 FROM local_credentials c JOIN users u ON u.id = c.user_id WHERE u.email_normalized = $1);
