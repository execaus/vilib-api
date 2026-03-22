-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.account_permissions RENAME TO account_status;
ALTER TABLE app.account_status RENAME COLUMN permission TO status;

COMMENT ON TABLE app.account_status IS 'Таблица для хранения статусов пользовательских аккаунтов';
COMMENT ON COLUMN app.account_status.status IS 'Статус аккаунта';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.account_status RENAME TO account_permissions;
ALTER TABLE app.account_permissions RENAME COLUMN status TO permission;
-- +goose StatementEnd
