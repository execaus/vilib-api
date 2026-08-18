-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.watch_progress(
    user_id uuid not null
        constraint fk_watch_progress_user_id references app.users(user_id) on delete cascade,
    video_id uuid not null
        constraint fk_watch_progress_video_id references app.user_group_videos(id) on delete cascade,
    intervals int8multirange not null default '{}',
    covered_ms bigint not null default 0,
    last_position_ms bigint not null default 0,
    wall_ms bigint not null default 0,
    first_at timestamp not null,
    last_at timestamp not null,
    threshold_reached_at timestamp null,
    primary key (user_id, video_id)
);

COMMENT ON TABLE app.watch_progress IS 'Прогресс просмотра видео пользователем — объединённые интервалы и накопленные метрики, независимо от назначений';

COMMENT ON COLUMN app.watch_progress.user_id IS 'Пользователь, чей прогресс учитывается';
COMMENT ON COLUMN app.watch_progress.video_id IS 'Видео, по которому учитывается прогресс';
COMMENT ON COLUMN app.watch_progress.intervals IS 'Объединённые просмотренные интервалы [from_ms, to_ms) видео';
COMMENT ON COLUMN app.watch_progress.covered_ms IS 'Суммарная длина intervals в миллисекундах (денормализация для чтения без unnest)';
COMMENT ON COLUMN app.watch_progress.last_position_ms IS 'Позиция плеера для продолжения просмотра';
COMMENT ON COLUMN app.watch_progress.wall_ms IS 'Накопленное реальное (астрономическое) время просмотра — защита от накрутки';
COMMENT ON COLUMN app.watch_progress.first_at IS 'Время первого принятого heartbeat''а';
COMMENT ON COLUMN app.watch_progress.last_at IS 'Время последнего принятого heartbeat''а';
COMMENT ON COLUMN app.watch_progress.threshold_reached_at IS 'Момент первого достижения порога засчитывания (NULL — ещё не достигнут); нужен для зачёта при позднем назначении на уже просмотренное видео';

CREATE TABLE app.watch_sessions(
    session_id uuid primary key,
    user_id uuid not null
        constraint fk_watch_sessions_user_id references app.users(user_id) on delete cascade,
    video_id uuid not null
        constraint fk_watch_sessions_video_id references app.user_group_videos(id) on delete cascade,
    last_seq int not null,
    started_at timestamp not null default now(),
    last_at timestamp not null,
    last_position_ms bigint not null default 0
);

COMMENT ON TABLE app.watch_sessions IS 'Сессии просмотра — идемпотентность heartbeat''ов и защита от перемотки в рамках одной сессии плеера';

COMMENT ON COLUMN app.watch_sessions.session_id IS 'Идентификатор сессии (генерирует клиент при открытии плеера)';
COMMENT ON COLUMN app.watch_sessions.user_id IS 'Пользователь, которому принадлежит сессия';
COMMENT ON COLUMN app.watch_sessions.video_id IS 'Видео, которое смотрят в рамках сессии';
COMMENT ON COLUMN app.watch_sessions.last_seq IS 'Порядковый номер последнего принятого heartbeat''а сессии (идемпотентность)';
COMMENT ON COLUMN app.watch_sessions.started_at IS 'Время открытия сессии';
COMMENT ON COLUMN app.watch_sessions.last_at IS 'Время последнего принятого heartbeat''а сессии — база для расчёта допустимой длины следующего интервала';
COMMENT ON COLUMN app.watch_sessions.last_position_ms IS 'Последняя позиция плеера, полученная в рамках сессии';

CREATE INDEX idx_watch_sessions_user_id_video_id ON app.watch_sessions(user_id, video_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.watch_sessions;
DROP TABLE IF EXISTS app.watch_progress;
-- +goose StatementEnd
