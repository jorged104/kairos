-- Spec 020: informes de proyecto con IA.

-- name: SetAISettings :exec
UPDATE system_settings SET
    ai_enabled = $1, ai_base_url = $2, ai_api_key = $3, ai_model = $4,
    version = version + 1, updated_at = now()
WHERE id;

-- name: ListReportTemplates :many
-- Every template, the active ones first (RF-18).
SELECT * FROM report_templates ORDER BY archived_at IS NOT NULL, name;

-- name: ActiveReportTemplates :many
SELECT * FROM report_templates WHERE archived_at IS NULL ORDER BY name;

-- name: GetReportTemplate :one
SELECT * FROM report_templates WHERE id = $1;

-- name: CreateReportTemplate :one
INSERT INTO report_templates (name, name_normalized, description, prompt)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateReportTemplate :one
UPDATE report_templates SET name = $2, name_normalized = $3, description = $4, prompt = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveReportTemplate :one
UPDATE report_templates SET archived_at = now(), updated_at = now()
WHERE id = $1 AND archived_at IS NULL
RETURNING *;

-- name: ActiveReportTemplateNameTaken :one
SELECT EXISTS (
    SELECT 1 FROM report_templates
    WHERE name_normalized = sqlc.arg(name_normalized)::text AND archived_at IS NULL
      AND id IS DISTINCT FROM sqlc.narg(except_id)::uuid
);

-- name: ListProjectReports :many
-- The reports of a project, the most recently changed first (RF-29, RF-30).
SELECT r.id, r.project_id, r.template_id, r.template_name, r.title, r.author_id, a.name AS author_name,
       r.updated_by, u.name AS updated_by_name, r.created_at, r.updated_at, r.version
FROM project_reports r
JOIN users a ON a.id = r.author_id
JOIN users u ON u.id = r.updated_by
WHERE r.project_id = $1
ORDER BY r.updated_at DESC, r.id;

-- name: GetProjectReport :one
SELECT r.*, a.name AS author_name, u.name AS updated_by_name
FROM project_reports r
JOIN users a ON a.id = r.author_id
JOIN users u ON u.id = r.updated_by
WHERE r.id = $1;

-- name: GetProjectReportForUpdate :one
SELECT * FROM project_reports WHERE id = $1 FOR UPDATE;

-- name: CreateProjectReport :one
INSERT INTO project_reports (project_id, template_id, template_name, title, content, author_id, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $6)
RETURNING id;

-- name: UpdateProjectReport :exec
UPDATE project_reports SET title = $2, content = $3, updated_by = $4, updated_at = now(), version = version + 1
WHERE id = $1;

-- name: DeleteProjectReport :exec
DELETE FROM project_reports WHERE id = $1;
