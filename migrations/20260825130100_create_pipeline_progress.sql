-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.pipeline_progress(
    is_urgent boolean primary key,
    last_dequeued_at timestamp not null default now()
);

COMMENT ON TABLE app.pipeline_progress IS
    'Индикатор живости конвейера обработки видео по полосам (архивная/срочная) — ровно две строки, обновляется при каждом успешном взятии видео в обработку (эпик Э5, исправление Д-1)';

COMMENT ON COLUMN app.pipeline_progress.is_urgent IS 'Полоса обработки: false — архивная, true — срочная';
COMMENT ON COLUMN app.pipeline_progress.last_dequeued_at IS
    'Момент последнего успешно обработанного события ProcessingStarted в этой полосе';

INSERT INTO app.pipeline_progress (is_urgent) VALUES (false), (true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.pipeline_progress;
-- +goose StatementEnd
