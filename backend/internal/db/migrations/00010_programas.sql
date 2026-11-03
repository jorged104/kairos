-- +goose Up
-- Spec 010: programas (RF-1, RF-2) y peso de cada proyecto en ellos (RF-5,
-- RF-6). Solo aditiva.
ALTER TABLE projects
    ADD COLUMN is_program       boolean NOT NULL DEFAULT false,
    ADD COLUMN portfolio_weight int CHECK (portfolio_weight BETWEEN 0 AND 100000);

-- +goose Down
ALTER TABLE projects DROP COLUMN portfolio_weight, DROP COLUMN is_program;
