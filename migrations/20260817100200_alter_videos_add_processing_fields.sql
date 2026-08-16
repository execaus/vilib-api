-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.user_group_videos
    ADD COLUMN status_changed_at timestamp NOT NULL DEFAULT now(),
    ADD COLUMN processing_attempt int NOT NULL DEFAULT 0,
    ADD COLUMN failure_class varchar NULL,
    ADD COLUMN failure_reason varchar NULL,
    ADD COLUMN duration_ms bigint NULL,
    ADD COLUMN width int NULL,
    ADD COLUMN height int NULL;

CREATE INDEX IF NOT EXISTS idx_user_group_videos_status_status_changed_at
    ON app.user_group_videos (status, status_changed_at);

COMMENT ON COLUMN app.user_group_videos.status_changed_at IS 'Время последнего изменения статуса';
COMMENT ON COLUMN app.user_group_videos.processing_attempt IS 'Номер текущей попытки обработки';
COMMENT ON COLUMN app.user_group_videos.failure_class IS 'Класс ошибки обработки: permanent, temporary, timeout';
COMMENT ON COLUMN app.user_group_videos.failure_reason IS 'Человекочитаемая причина ошибки обработки';
COMMENT ON COLUMN app.user_group_videos.duration_ms IS 'Длительность видео в миллисекундах';
COMMENT ON COLUMN app.user_group_videos.width IS 'Ширина видео в пикселях';
COMMENT ON COLUMN app.user_group_videos.height IS 'Высота видео в пикселях';
COMMENT ON COLUMN app.user_group_videos.status IS 'Статус видео: 0 uploading, 1 compressing, 2 ready, 3 failed, 4 queued';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_user_group_videos_status_status_changed_at;

ALTER TABLE app.user_group_videos
    DROP COLUMN IF EXISTS status_changed_at,
    DROP COLUMN IF EXISTS processing_attempt,
    DROP COLUMN IF EXISTS failure_class,
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS duration_ms,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS height;

COMMENT ON COLUMN app.user_group_videos.status IS 'Статус видео';
-- +goose StatementEnd
