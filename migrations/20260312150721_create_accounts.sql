-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.accounts(
    account_id uuid primary key default gen_random_uuid(),
    name varchar not null,
    owner_id uuid not null,
    email varchar not null,
    created_at timestamp not null default time_now()
);

COMMENT ON TABLE app.accounts IS 'Аккаунты организаций';

COMMENT ON COLUMN app.accounts.account_id IS 'Идентификатор аккаунта';
COMMENT ON COLUMN app.accounts.name IS 'Отображаемое название аккаунта';
COMMENT ON COLUMN app.accounts.owner_id IS 'Идентификатор пользователя — владельца аккаунта';
COMMENT ON COLUMN app.accounts.email IS 'Контактный email, связанный с аккаунтом';
COMMENT ON COLUMN app.accounts.created_at IS 'Время создания записи аккаунта';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.accounts;
-- +goose StatementEnd