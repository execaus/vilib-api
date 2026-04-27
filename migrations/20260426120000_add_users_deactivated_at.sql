-- +goose Up
ALTER TABLE app.users ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMP NULL;

-- +goose Down
ALTER TABLE app.users DROP COLUMN IF EXISTS deactivated_at;
