-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.users(
    user_id uuid primary key default gen_random_uuid(),
    name varchar not null,
    surname varchar not null,
    password_hash varchar not null,
    email varchar not null,
    created_at timestamp not null default now()
);

COMMENT ON TABLE app.users IS 'Пользователи';

COMMENT ON COLUMN app.users.user_id IS 'Идентификатор пользователя';
COMMENT ON COLUMN app.users.name IS 'Отображаемое имя пользователя';
COMMENT ON COLUMN app.users.surname IS 'Отображаемая фамилия пользователя';
COMMENT ON COLUMN app.users.password_hash IS 'Хеш пароля пользователя';
COMMENT ON COLUMN app.users.email IS 'Контактный email, связанный с пользователем';
COMMENT ON COLUMN app.users.created_at IS 'Время создания записи пользователя';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.users;
-- +goose StatementEnd