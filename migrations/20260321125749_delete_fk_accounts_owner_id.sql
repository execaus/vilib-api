-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.accounts
    DROP CONSTRAINT fk_owner_id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.accounts
    ADD CONSTRAINT fk_owner_id
        FOREIGN KEY (owner_id) REFERENCES app.users(user_id);
-- +goose StatementEnd