-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.video_chapters(
    chapter_id uuid primary key default gen_random_uuid(),
    video_id uuid not null
        constraint fk_video_chapters_video_id references app.user_group_videos(id) on delete cascade,
    start_ms bigint not null
        constraint chk_video_chapters_start_ms check (start_ms >= 0),
    name varchar(200) not null
        constraint chk_video_chapters_name check (char_length(name) between 1 and 200),
    created_at timestamp not null default now(),
    UNIQUE(video_id, start_ms)
);

COMMENT ON TABLE app.video_chapters IS 'Главы видео — точки начала разделов ролика; конец главы не хранится, а вычисляется как начало следующей главы либо длительность видео (эпик Э4)';

COMMENT ON COLUMN app.video_chapters.chapter_id IS 'Идентификатор главы';
COMMENT ON COLUMN app.video_chapters.video_id IS 'Видео, которому принадлежит глава; при удалении видео главы удаляются каскадом';
COMMENT ON COLUMN app.video_chapters.start_ms IS 'Момент начала главы в миллисекундах от начала ролика; конец главы вычисляется в SQL как начало следующей главы либо длительность видео';
COMMENT ON COLUMN app.video_chapters.name IS 'Название главы (1–200 символов)';
COMMENT ON COLUMN app.video_chapters.created_at IS 'Время создания главы';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.video_chapters;
-- +goose StatementEnd
