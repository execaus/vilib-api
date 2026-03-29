-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.video_assets
    ADD CONSTRAINT fk_video_assets_file
        FOREIGN KEY (file_id)
            REFERENCES app.files(file_id)
            ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.video_assets
    DROP CONSTRAINT IF EXISTS fk_video_assets_file;
-- +goose StatementEnd
