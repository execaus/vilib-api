-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS app.user_groups(
    group_id uuid primary key default gen_random_uuid(),
    name varchar not null,
    account_id uuid not null,
    UNIQUE(name, account_id)
);

COMMENT ON TABLE app.user_groups IS 'Группы пользователей';

COMMENT ON COLUMN app.user_groups.group_id IS 'Идентификатор группы';
COMMENT ON COLUMN app.user_groups.name IS 'Название группы';
COMMENT ON COLUMN app.user_groups.account_id IS 'Идентификатор аккаунта';

ALTER TABLE app.user_groups ADD CONSTRAINT fk_user_groups_account_id FOREIGN KEY (account_id) REFERENCES app.accounts(account_id);

CREATE TABLE IF NOT EXISTS app.group_members(
    user_id uuid not null,
    group_id uuid not null,
    role_id uuid not null,
    primary key (user_id, group_id)
);

COMMENT ON TABLE app.group_members IS 'Связь пользователей с группами';

COMMENT ON COLUMN app.group_members.user_id IS 'Идентификатор пользователя';
COMMENT ON COLUMN app.group_members.group_id IS 'Идентификатор группы';
COMMENT ON COLUMN app.group_members.role_id IS 'Идентификатор роли пользователя в группе';

CREATE TABLE IF NOT EXISTS app.group_roles(
    group_role_id uuid primary key default gen_random_uuid(),
    name varchar not null,
    permissions bigint not null,
    account_id uuid not null,
    UNIQUE(account_id, name)
);

COMMENT ON TABLE app.group_roles IS 'Роли групп с набором прав';

COMMENT ON COLUMN app.group_roles.group_role_id IS 'Идентификатор роли';
COMMENT ON COLUMN app.group_roles.name IS 'Название роли';
COMMENT ON COLUMN app.group_roles.permissions IS 'Набор прав в виде 64-битной маски';
COMMENT ON COLUMN app.group_roles.account_id IS 'Идентификатор аккаунта';

ALTER TABLE app.group_roles ADD CONSTRAINT fk_group_role_account_id FOREIGN KEY (account_id) REFERENCES app.accounts(account_id);

ALTER TABLE app.group_members ADD CONSTRAINT fk_group_members_group_id FOREIGN KEY (group_id) REFERENCES app.user_groups(group_id);
ALTER TABLE app.group_members ADD CONSTRAINT fk_group_members_user_id FOREIGN KEY (user_id) REFERENCES app.users(user_id);
ALTER TABLE app.group_members ADD CONSTRAINT fk_group_members_role_id FOREIGN KEY (role_id) REFERENCES app.group_roles(group_role_id);

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

ALTER TABLE app.group_members DROP CONSTRAINT IF EXISTS fk_group_members_role_id;
ALTER TABLE app.group_members DROP CONSTRAINT IF EXISTS fk_group_members_group_id;
ALTER TABLE app.group_members DROP CONSTRAINT IF EXISTS fk_group_members_user_id;

ALTER TABLE app.user_groups DROP CONSTRAINT IF EXISTS fk_user_groups_account_id;
ALTER TABLE app.group_roles DROP CONSTRAINT IF EXISTS fk_group_role_account_id;

DROP TABLE IF EXISTS app.group_members;
DROP TABLE IF EXISTS app.group_roles;
DROP TABLE IF EXISTS app.user_groups;

-- +goose StatementEnd
