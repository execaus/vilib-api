-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.user_group_videos(
    id uuid primary key default gen_random_uuid(),
    user_group_id uuid not null,
    name varchar not null,
    author uuid not null references app.users(user_id),
    status int not null,
    created_at timestamp not null default now(),
    UNIQUE(user_group_id, name)
);

COMMENT ON TABLE app.user_group_videos IS 'Видео, принадлежащие группам пользователей';

COMMENT ON COLUMN app.user_group_videos.id IS 'Идентификатор видео';
COMMENT ON COLUMN app.user_group_videos.user_group_id IS 'Идентификатор группы пользователей';
COMMENT ON COLUMN app.user_group_videos.name IS 'Название видео';
COMMENT ON COLUMN app.user_group_videos.author IS 'Автор (пользователь)';
COMMENT ON COLUMN app.user_group_videos.status IS 'Статус видео';
COMMENT ON COLUMN app.user_group_videos.created_at IS 'Время создания записи';

CREATE TABLE IF NOT EXISTS app.video_assets(
    file_id uuid primary key,
    video_id uuid not null references app.user_group_videos(id),
    tag int not null,
    created_at timestamp not null default now()
);

COMMENT ON TABLE app.video_assets IS 'Связь видео с файлами';

COMMENT ON COLUMN app.video_assets.file_id IS 'Идентификатор файла';
COMMENT ON COLUMN app.video_assets.video_id IS 'Идентификатор видео';
COMMENT ON COLUMN app.video_assets.tag IS 'Тег видео';
COMMENT ON COLUMN app.video_assets.created_at IS 'Время создания записи';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.video_assets;
DROP TABLE IF EXISTS app.user_group_videos;
-- +goose StatementEnd
