-- +goose Up
-- Spec 024: la línea base guarda la clave y el título de cada tarea y conserva
-- las tareas borradas (RF-24, RF-25). Solo aditiva: la versión anterior de la
-- API lee las fechas cruzando con work_items, así que una tarea borrada sigue
-- sin contar para ella.
ALTER TABLE project_baseline_items
    ADD COLUMN item_key text,
    ADD COLUMN title    text;
ALTER TABLE project_baseline_items DROP CONSTRAINT project_baseline_items_work_item_id_fkey;

-- +goose Down
DELETE FROM project_baseline_items bi WHERE NOT EXISTS (SELECT 1 FROM work_items w WHERE w.id = bi.work_item_id);
ALTER TABLE project_baseline_items ADD CONSTRAINT project_baseline_items_work_item_id_fkey
    FOREIGN KEY (work_item_id) REFERENCES work_items (id) ON DELETE CASCADE;
ALTER TABLE project_baseline_items DROP COLUMN title, DROP COLUMN item_key;
