-- Spec 010: programas y portafolio.

-- name: SetProjectPortfolio :one
UPDATE projects SET is_program = $2, portfolio_weight = $3, version = version + 1, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ProjectEffort :one
-- The sum of the weights of the project's own work items (Conceptos).
SELECT coalesce(sum(weight), 0)::int FROM work_items WHERE project_id = $1;

-- name: PortfolioProjects :many
-- Every project but templates, archived ones too to walk the tree, with its
-- effort.
SELECT p.id, p.parent_id, p.identifier, p.name, p.lifecycle, p.is_program, p.portfolio_weight,
       p.start_date, p.end_date, p.team_id, p.owner_id,
       coalesce((SELECT sum(w.weight) FROM work_items w WHERE w.project_id = p.id), 0)::int AS effort
FROM projects p
WHERE NOT p.is_template AND NOT p.is_inbox
ORDER BY p.name, p.identifier;

-- name: PortfolioPlanItems :many
-- The work items of the projects not archived, as ProjectPlanItems.
SELECT w.project_id, w.id, w.parent_id, w.weight, s.is_closed,
       (CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END)::int AS ratio,
       w.start_date, w.due_date,
       (bi.work_item_id IS NOT NULL)::bool AS in_baseline,
       bi.start_date AS base_start, bi.due_date AS base_due
FROM work_items w
JOIN projects p ON p.id = w.project_id AND NOT p.is_template AND NOT p.is_inbox AND p.lifecycle <> 'archivado'
JOIN statuses s ON s.id = w.status_id
LEFT JOIN project_baseline_items bi ON bi.work_item_id = w.id AND bi.baseline_id = (
    SELECT b.id FROM project_baselines b WHERE b.project_id = w.project_id ORDER BY b.version DESC LIMIT 1);
