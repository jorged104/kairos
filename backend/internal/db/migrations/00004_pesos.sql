-- +goose Up
-- Spec 004, pesos (RF-149 a RF-159): peso por work item, peso por defecto por
-- tipo y avance derivado ponderado por peso. Solo aditiva: derived_done_ratio
-- sigue al día para la versión anterior de la API y se borra más adelante.

ALTER TABLE work_item_types
    ADD COLUMN default_weight smallint NOT NULL DEFAULT 1 CHECK (default_weight BETWEEN 0 AND 100);

ALTER TABLE work_items
    ADD COLUMN weight smallint NOT NULL DEFAULT 1 CHECK (weight BETWEEN 0 AND 100),
    -- Subtree sums, the node included: weights and weight × done ratio (closed = 100).
    ADD COLUMN agg_weight           bigint NOT NULL DEFAULT 1,
    ADD COLUMN agg_weight_ratio_sum bigint NOT NULL DEFAULT 0;

-- A subtree where everything weighs 0 falls back to the simple mean (RF-153).
ALTER TABLE work_items ADD COLUMN weighted_done_ratio smallint GENERATED ALWAYS AS (
    CASE WHEN agg_weight > 0 THEN round(agg_weight_ratio_sum::numeric / agg_weight)::smallint
         ELSE round(agg_ratio_sum::numeric / agg_count)::smallint END
) STORED;

-- +goose StatementBegin
-- Same signature as before, so the previous API keeps calling it; it now also
-- keeps the weight sums up to date.
CREATE OR REPLACE FUNCTION work_item_refresh_one(p_id uuid) RETURNS uuid AS $$
    UPDATE work_items w SET
        (agg_estimated, agg_remaining, agg_count, agg_ratio_sum, agg_est_ratio_sum,
         agg_weight, agg_weight_ratio_sum) = (
            SELECT coalesce(w.estimated_hours, 0) + coalesce(sum(k.agg_estimated), 0),
                   coalesce(w.remaining_hours, 0) + coalesce(sum(k.agg_remaining), 0),
                   1 + coalesce(sum(k.agg_count), 0)::int,
                   r.ratio + coalesce(sum(k.agg_ratio_sum), 0)::bigint,
                   coalesce(w.estimated_hours, 0) * r.ratio + coalesce(sum(k.agg_est_ratio_sum), 0),
                   w.weight + coalesce(sum(k.agg_weight), 0)::bigint,
                   w.weight * r.ratio + coalesce(sum(k.agg_weight_ratio_sum), 0)::bigint
            FROM (SELECT CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END AS ratio
                  FROM statuses s WHERE s.id = w.status_id) r
            LEFT JOIN work_items k ON k.parent_id = w.id
            GROUP BY r.ratio
        )
    WHERE w.id = p_id
    RETURNING w.parent_id;
$$ LANGUAGE sql;
-- +goose StatementEnd

SELECT work_item_recompute_project(id) FROM projects;

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION work_item_refresh_one(p_id uuid) RETURNS uuid AS $$
    UPDATE work_items w SET
        (agg_estimated, agg_remaining, agg_count, agg_ratio_sum, agg_est_ratio_sum) = (
            SELECT coalesce(w.estimated_hours, 0) + coalesce(sum(k.agg_estimated), 0),
                   coalesce(w.remaining_hours, 0) + coalesce(sum(k.agg_remaining), 0),
                   1 + coalesce(sum(k.agg_count), 0)::int,
                   r.ratio + coalesce(sum(k.agg_ratio_sum), 0)::bigint,
                   coalesce(w.estimated_hours, 0) * r.ratio + coalesce(sum(k.agg_est_ratio_sum), 0)
            FROM (SELECT CASE WHEN s.is_closed THEN 100 ELSE w.done_ratio END AS ratio
                  FROM statuses s WHERE s.id = w.status_id) r
            LEFT JOIN work_items k ON k.parent_id = w.id
            GROUP BY r.ratio
        )
    WHERE w.id = p_id
    RETURNING w.parent_id;
$$ LANGUAGE sql;
-- +goose StatementEnd
ALTER TABLE work_items DROP COLUMN weighted_done_ratio, DROP COLUMN agg_weight_ratio_sum,
    DROP COLUMN agg_weight, DROP COLUMN weight;
ALTER TABLE work_item_types DROP COLUMN default_weight;
