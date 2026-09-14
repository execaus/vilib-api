-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.accounts DROP CONSTRAINT IF EXISTS accounts_name_key;

COMMENT ON COLUMN app.accounts.name IS 'Название организации, указанное при регистрации; не уникально, организация идентифицируется по account_id';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.accounts ADD CONSTRAINT accounts_name_key UNIQUE (name);

COMMENT ON COLUMN app.accounts.name IS 'Отображаемое название аккаунта';
-- +goose StatementEnd
