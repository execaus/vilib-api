-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.account_roles ADD COLUMN is_default BOOLEAN NOT NULL default false;
ALTER TABLE app.group_roles ADD COLUMN is_default BOOLEAN NOT NULL default false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.account_roles DROP COLUMN is_default;
ALTER TABLE app.group_roles DROP COLUMN is_default;
-- +goose StatementEnd
