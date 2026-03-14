-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.account_permissions(
    user_id uuid not null references app.users(user_id) on delete cascade,
    account_id uuid not null references app.accounts(account_id) on delete cascade,
    permission int4 not null,
    updated_at timestamp not null default now(),
    primary key (user_id, account_id)
);

COMMENT ON TABLE app.account_permissions IS 'Права пользователей на аккаунты';

COMMENT ON COLUMN app.account_permissions.user_id IS 'Идентификатор пользователя';
COMMENT ON COLUMN app.account_permissions.account_id IS 'Идентификатор аккаунта';
COMMENT ON COLUMN app.account_permissions.permission IS 'Битовая маска разрешений пользователя для аккаунта. 32-битное целое число, где каждый бит соответствует отдельному праву доступа';
COMMENT ON COLUMN app.account_permissions.updated_at IS 'Время обновления записи разрешения';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.account_permissions;
-- +goose StatementEnd
