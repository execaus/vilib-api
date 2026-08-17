-- +goose Up
-- +goose StatementBegin
CREATE INDEX idx_users_email ON app.users(email);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_users_email;
-- +goose StatementEnd
