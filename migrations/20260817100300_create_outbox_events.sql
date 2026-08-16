-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.outbox_events(
    id bigserial primary key,
    topic varchar not null,
    key varchar not null,
    payload jsonb not null,
    created_at timestamp not null default now()
);

COMMENT ON TABLE app.outbox_events IS 'Исходящие события, ожидающие публикации в брокер сообщений';

COMMENT ON COLUMN app.outbox_events.id IS 'Идентификатор события, определяет порядок публикации';
COMMENT ON COLUMN app.outbox_events.topic IS 'Топик брокера сообщений';
COMMENT ON COLUMN app.outbox_events.key IS 'Ключ партиционирования (video_id)';
COMMENT ON COLUMN app.outbox_events.payload IS 'Сериализованный конверт события';
COMMENT ON COLUMN app.outbox_events.created_at IS 'Время создания события';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.outbox_events;
-- +goose StatementEnd
