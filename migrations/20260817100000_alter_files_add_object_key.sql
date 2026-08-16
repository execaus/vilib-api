-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.files
    ADD COLUMN object_key varchar NOT NULL;

ALTER TABLE app.files
    ADD CONSTRAINT files_bucket_object_key_key UNIQUE (bucket, object_key);

COMMENT ON COLUMN app.files.object_key IS 'Ключ объекта в бакете хранилища';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.files
    DROP CONSTRAINT IF EXISTS files_bucket_object_key_key;

ALTER TABLE app.files
    DROP COLUMN IF EXISTS object_key;
-- +goose StatementEnd
