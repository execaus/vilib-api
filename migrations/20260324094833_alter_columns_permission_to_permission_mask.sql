-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.group_roles
RENAME COLUMN permissions TO permission_mask;

ALTER TABLE app.account_roles
RENAME COLUMN permissions TO permission_mask;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.group_roles
RENAME COLUMN permission_mask TO permissions;

ALTER TABLE app.account_roles
RENAME COLUMN permission_mask TO permissions;
-- +goose StatementEnd
