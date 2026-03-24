-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.account_roles
ADD COLUMN parent_role_id uuid NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE app.account_roles
ADD CONSTRAINT fk_parent_role_id
FOREIGN KEY (parent_role_id) REFERENCES app.account_roles(account_role_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.account_roles
DROP CONSTRAINT fk_parent_role_id;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE app.account_roles
DROP COLUMN parent_role_id;
-- +goose StatementEnd
