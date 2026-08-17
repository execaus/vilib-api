-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.group_members
    ADD COLUMN joined_at timestamp NOT NULL DEFAULT now();

COMMENT ON COLUMN app.group_members.joined_at IS 'Время добавления участника в группу';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.group_members
    DROP COLUMN IF EXISTS joined_at;
-- +goose StatementEnd
