-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.password_reset_tokens(
    token_id uuid primary key default gen_random_uuid(),
    user_id uuid not null
        constraint fk_password_reset_tokens_user_id references app.users(user_id) on delete cascade,
    email varchar not null,
    token_hash varchar not null unique,
    expires_at timestamp not null,
    used_at timestamp null,
    created_at timestamp not null default now()
);

CREATE INDEX idx_password_reset_tokens_email ON app.password_reset_tokens(email);

COMMENT ON TABLE app.password_reset_tokens IS 'Одноразовые токены сброса пароля (хранится только хеш)';

COMMENT ON COLUMN app.password_reset_tokens.token_id IS 'Идентификатор токена';
COMMENT ON COLUMN app.password_reset_tokens.user_id IS 'Строка пользователя, чей пароль будет изменён (пароль на организацию)';
COMMENT ON COLUMN app.password_reset_tokens.email IS 'Email, на который запрошен сброс — для удаления прежних токенов';
COMMENT ON COLUMN app.password_reset_tokens.token_hash IS 'SHA-256 хеш токена (сырой токен уходит только в письме)';
COMMENT ON COLUMN app.password_reset_tokens.expires_at IS 'Время истечения срока действия токена';
COMMENT ON COLUMN app.password_reset_tokens.used_at IS 'Время использования токена (NULL — ещё не использован)';
COMMENT ON COLUMN app.password_reset_tokens.created_at IS 'Время создания токена';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.password_reset_tokens;
-- +goose StatementEnd
