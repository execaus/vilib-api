-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.accounts
    DROP COLUMN IF EXISTS owner_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.accounts
    ADD COLUMN owner_id uuid NOT NULL;

COMMENT ON COLUMN app.accounts.owner_id IS 'Идентификатор пользователя — владельца аккаунта';
-- +goose StatementEnd