-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.user_group_videos
    ADD COLUMN is_urgent boolean NOT NULL DEFAULT false,
    ADD COLUMN queued_at timestamp NULL,
    ADD COLUMN compressing_started_at timestamp NULL,
    ADD COLUMN ready_at timestamp NULL;

DROP INDEX IF EXISTS app.idx_user_group_videos_status_status_changed_at;

CREATE INDEX IF NOT EXISTS idx_user_group_videos_status_is_urgent_status_changed_at
    ON app.user_group_videos (status, is_urgent, status_changed_at);

COMMENT ON COLUMN app.user_group_videos.is_urgent IS
    'Признак срочного видео: берётся в обработку приоритетной полосой мимо общей очереди (эпик Э5)';
COMMENT ON COLUMN app.user_group_videos.queued_at IS
    'Время постановки в очередь на обработку (переход uploading → queued) — момент complete из метрики публикации';
COMMENT ON COLUMN app.user_group_videos.compressing_started_at IS
    'Время взятия в обработку конвейером (переход queued → compressing по событию ProcessingStarted)';
COMMENT ON COLUMN app.user_group_videos.ready_at IS
    'Время готовности видео (переход compressing → ready по событию ProcessingCompleted)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_user_group_videos_status_is_urgent_status_changed_at;

CREATE INDEX IF NOT EXISTS idx_user_group_videos_status_status_changed_at
    ON app.user_group_videos (status, status_changed_at);

ALTER TABLE app.user_group_videos
    DROP COLUMN IF EXISTS is_urgent,
    DROP COLUMN IF EXISTS queued_at,
    DROP COLUMN IF EXISTS compressing_started_at,
    DROP COLUMN IF EXISTS ready_at;
-- +goose StatementEnd
