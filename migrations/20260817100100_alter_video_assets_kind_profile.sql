-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.video_assets
    DROP CONSTRAINT IF EXISTS video_assets_video_id_fkey;

ALTER TABLE app.video_assets
    DROP COLUMN tag;

ALTER TABLE app.video_assets
    ADD COLUMN kind varchar NOT NULL DEFAULT 'original',
    ADD COLUMN profile varchar NOT NULL DEFAULT '';

ALTER TABLE app.video_assets
    ALTER COLUMN kind DROP DEFAULT;

ALTER TABLE app.video_assets
    ADD CONSTRAINT video_assets_video_id_kind_profile_key UNIQUE (video_id, kind, profile);

ALTER TABLE app.video_assets
    ADD CONSTRAINT video_assets_video_id_fkey
        FOREIGN KEY (video_id)
            REFERENCES app.user_group_videos(id)
            ON DELETE CASCADE;

COMMENT ON COLUMN app.video_assets.kind IS 'Вид ассета: original, hls_master, hls_variant';
COMMENT ON COLUMN app.video_assets.profile IS 'Профиль качества (360p/720p/1080p) для hls_variant, пустая строка для остальных';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.video_assets
    DROP CONSTRAINT IF EXISTS video_assets_video_id_fkey;

ALTER TABLE app.video_assets
    DROP CONSTRAINT IF EXISTS video_assets_video_id_kind_profile_key;

ALTER TABLE app.video_assets
    DROP COLUMN kind,
    DROP COLUMN profile;

ALTER TABLE app.video_assets
    ADD COLUMN tag int NOT NULL DEFAULT 0;

ALTER TABLE app.video_assets
    ALTER COLUMN tag DROP DEFAULT;

ALTER TABLE app.video_assets
    ADD CONSTRAINT video_assets_video_id_fkey
        FOREIGN KEY (video_id)
            REFERENCES app.user_group_videos(id);

COMMENT ON COLUMN app.video_assets.tag IS 'Тег видео';
-- +goose StatementEnd
