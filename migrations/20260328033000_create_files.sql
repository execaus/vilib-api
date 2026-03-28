-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.files(
    file_id uuid primary key default gen_random_uuid(),
    bucket varchar not null,
    content_type varchar not null,
    size_bytes bigint not null,
    created_at timestamp not null default now()
);

COMMENT ON TABLE app.files IS 'Файлы';

COMMENT ON COLUMN app.files.file_id IS 'Идентификатор файла';
COMMENT ON COLUMN app.files.bucket IS 'Бакет хранилища';
COMMENT ON COLUMN app.files.content_type IS 'MIME-тип файла';
COMMENT ON COLUMN app.files.size_bytes IS 'Размер файла в байтах';
COMMENT ON COLUMN app.files.created_at IS 'Время создания файла';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.files;
-- +goose StatementEnd
