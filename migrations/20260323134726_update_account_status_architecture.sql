-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.account_roles (
    account_role_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar NOT NULL,
    permissions bigint NOT NULL,
    account_id uuid NOT NULL,
    CONSTRAINT unique_account_role UNIQUE (account_id, name)
);

ALTER TABLE app.account_roles
    ADD CONSTRAINT fk_account FOREIGN KEY (account_id) REFERENCES app.accounts(account_id);

COMMENT ON TABLE app.account_roles IS 'Таблица, хранящая роли, назначенные аккаунтам';

COMMENT ON COLUMN app.account_roles.account_role_id IS 'Первичный ключ для роли аккаунта';
COMMENT ON COLUMN app.account_roles.name IS 'Название роли';
COMMENT ON COLUMN app.account_roles.permissions IS 'Битовая маска разрешений, связанная с ролью';
COMMENT ON COLUMN app.account_roles.account_id IS 'Ссылка на аккаунт, которому принадлежит роль';

ALTER TABLE app.users ADD COLUMN role_id uuid not null default gen_random_uuid();

ALTER TABLE app.users
    ADD CONSTRAINT fk_role FOREIGN KEY (role_id) REFERENCES app.account_roles(account_role_id);

DROP TABLE app.account_status;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE app.account_status(
    account_permission_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar NOT NULL,
    status bigint NOT NULL,
    account_id uuid NOT NULL,
    CONSTRAINT unique_account_status UNIQUE (account_id, name)
);

ALTER TABLE app.account_status
    ADD CONSTRAINT fk_account FOREIGN KEY (account_id) REFERENCES app.accounts(account_id);

COMMENT ON TABLE app.account_status IS 'Таблица, хранящая разрешения, назначенные аккаунтам';
COMMENT ON COLUMN app.account_status.account_permission_id IS 'Первичный ключ для разрешения аккаунта';
COMMENT ON COLUMN app.account_status.name IS 'Название разрешения';
COMMENT ON COLUMN app.account_status.status IS 'Битовая маска разрешения';
COMMENT ON COLUMN app.account_status.account_id IS 'Ссылка на аккаунт, которому принадлежит разрешение';

ALTER TABLE app.users DROP CONSTRAINT fk_role;
ALTER TABLE app.users DROP COLUMN role_id;

DROP TABLE app.account_roles;
-- +goose StatementEnd
