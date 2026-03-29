-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.account_roles
ADD COLUMN is_system BOOLEAN NOT NULL default false;
COMMENT ON COLUMN app.account_roles.is_system IS 'Признак того, что роль является системной';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.account_roles
DROP COLUMN is_system;
-- +goose StatementEnd
