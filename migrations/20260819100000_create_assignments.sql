-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.assignments(
    assignment_id uuid primary key default gen_random_uuid(),
    account_id uuid not null
        constraint fk_assignments_account_id references app.accounts(account_id),
    video_id uuid null
        constraint fk_assignments_video_id references app.user_group_videos(id) on delete set null,
    video_name varchar not null,
    group_id uuid null
        constraint fk_assignments_group_id references app.user_groups(group_id) on delete set null,
    group_name varchar not null,
    created_by uuid not null
        constraint fk_assignments_created_by references app.users(user_id),
    created_at timestamp not null default now(),
    due_mode varchar not null
        constraint chk_assignments_due_mode check (due_mode in ('date', 'days')),
    due_at timestamp null,
    due_days int null
        constraint chk_assignments_due_days check (due_days > 0),
    comment varchar(500) null,
    status varchar not null default 'active'
        constraint chk_assignments_status check (status in ('active', 'cancelled')),
    cancelled_at timestamp null,
    cancelled_by uuid null
        constraint fk_assignments_cancelled_by references app.users(user_id),
    cancel_reason varchar null
        constraint chk_assignments_cancel_reason check (cancel_reason in ('manual', 'video_deleted', 'group_deleted')),
    constraint chk_assignments_due check (
        (due_mode = 'date' and due_at is not null) or (due_mode = 'days' and due_days is not null)
    )
);

COMMENT ON TABLE app.assignments IS 'Назначения обязательного обучения (видео) сотрудникам и группам';

COMMENT ON COLUMN app.assignments.assignment_id IS 'Идентификатор назначения';
COMMENT ON COLUMN app.assignments.account_id IS 'Аккаунт-владелец назначения (изоляция арендатора)';
COMMENT ON COLUMN app.assignments.video_id IS 'Назначенное видео; NULL, если видео удалено (работает снимок video_name)';
COMMENT ON COLUMN app.assignments.video_name IS 'Снимок названия видео на момент создания назначения';
COMMENT ON COLUMN app.assignments.group_id IS 'Группа видео на момент создания назначения — область права на управление назначением; NULL, если группа удалена';
COMMENT ON COLUMN app.assignments.group_name IS 'Снимок названия группы на момент создания назначения';
COMMENT ON COLUMN app.assignments.created_by IS 'Пользователь, создавший назначение (без каскада — пользователей не удаляют, а деактивируют)';
COMMENT ON COLUMN app.assignments.created_at IS 'Время создания назначения';
COMMENT ON COLUMN app.assignments.due_mode IS 'Режим срока: date — фиксированная дата, days — количество дней с зачисления участника';
COMMENT ON COLUMN app.assignments.due_at IS 'Фиксированная дата срока (заполнена при due_mode=date)';
COMMENT ON COLUMN app.assignments.due_days IS 'Число дней с зачисления до срока (заполнено при due_mode=days)';
COMMENT ON COLUMN app.assignments.comment IS 'Комментарий назначившего (до 500 символов)';
COMMENT ON COLUMN app.assignments.status IS 'Статус назначения: active — действующее, cancelled — отменённое';
COMMENT ON COLUMN app.assignments.cancelled_at IS 'Время отмены назначения';
COMMENT ON COLUMN app.assignments.cancelled_by IS 'Пользователь, отменивший назначение';
COMMENT ON COLUMN app.assignments.cancel_reason IS 'Причина отмены: manual — вручную, video_deleted — удалено видео, group_deleted — удалена группа';

CREATE INDEX idx_assignments_account_id_status ON app.assignments(account_id, status);
CREATE INDEX idx_assignments_video_id ON app.assignments(video_id) WHERE video_id IS NOT NULL;
CREATE INDEX idx_assignments_group_id ON app.assignments(group_id);
CREATE INDEX idx_assignments_created_by ON app.assignments(created_by);

CREATE TABLE app.assignment_targets(
    assignment_id uuid not null
        constraint fk_assignment_targets_assignment_id references app.assignments(assignment_id) on delete cascade,
    target_type varchar not null
        constraint chk_assignment_targets_target_type check (target_type in ('user', 'group')),
    target_id uuid not null,
    primary key (assignment_id, target_type, target_id)
);

COMMENT ON TABLE app.assignment_targets IS 'Цели назначения (кому адресовано): конкретные пользователи или группа видео';

COMMENT ON COLUMN app.assignment_targets.assignment_id IS 'Назначение, к которому относится цель';
COMMENT ON COLUMN app.assignment_targets.target_type IS 'Тип цели: user — пользователь, group — группа';
COMMENT ON COLUMN app.assignment_targets.target_id IS 'Идентификатор пользователя или группы (без FK — полиморфная ссылка, цель остаётся историей после удаления)';

CREATE INDEX idx_assignment_targets_target ON app.assignment_targets(target_type, target_id);

CREATE TABLE app.assignment_participants(
    assignment_id uuid not null
        constraint fk_assignment_participants_assignment_id references app.assignments(assignment_id) on delete cascade,
    user_id uuid not null
        constraint fk_assignment_participants_user_id references app.users(user_id),
    status varchar not null
        constraint chk_assignment_participants_status check (status in ('assigned', 'in_progress', 'completed', 'cancelled')),
    source varchar not null
        constraint chk_assignment_participants_source check (source in ('personal', 'group')),
    source_group_id uuid null,
    enrolled_at timestamp not null default now(),
    due_at timestamp not null,
    completed_at timestamp null,
    completed_coverage_pct smallint null,
    completed_threshold_pct smallint null,
    completed_session_id uuid null,
    cancelled_at timestamp null,
    cancel_reason varchar null
        constraint chk_assignment_participants_cancel_reason check (
            cancel_reason in ('assignment_cancelled', 'removed_by_manager', 'left_group', 'video_deleted', 'group_deleted')
        ),
    primary key (assignment_id, user_id)
);

COMMENT ON TABLE app.assignment_participants IS 'Персональные записи участников назначения — прогресс и срок каждого сотрудника (PK гарантирует единственность записи на пару назначение-пользователь)';

COMMENT ON COLUMN app.assignment_participants.assignment_id IS 'Назначение, к которому относится участник';
COMMENT ON COLUMN app.assignment_participants.user_id IS 'Пользователь-участник';
COMMENT ON COLUMN app.assignment_participants.status IS 'Статус участника: assigned — назначено, in_progress — смотрит, completed — выполнено, cancelled — отменено';
COMMENT ON COLUMN app.assignment_participants.source IS 'Источник зачисления: personal — назначен лично, group — через членство в группе-цели';
COMMENT ON COLUMN app.assignment_participants.source_group_id IS 'Группа, через которую участник зачислен (без FK — история); заполнено при source=group';
COMMENT ON COLUMN app.assignment_participants.enrolled_at IS 'Момент зачисления (для новичков — момент добавления в группу)';
COMMENT ON COLUMN app.assignment_participants.due_at IS 'Персональный срок участника: вычислен при зачислении (date — из assignments.due_at, days — enrolled_at + due_days); денормализован для расчёта просрочки без JOIN';
COMMENT ON COLUMN app.assignment_participants.completed_at IS 'Момент подтверждения просмотра (неизменяемо после первой записи)';
COMMENT ON COLUMN app.assignment_participants.completed_coverage_pct IS 'Процент покрытия видео на момент подтверждения';
COMMENT ON COLUMN app.assignment_participants.completed_threshold_pct IS 'Порог засчитывания на момент подтверждения (версия правила)';
COMMENT ON COLUMN app.assignment_participants.completed_session_id IS 'Идентификатор сессии просмотра, при которой достигнут порог';
COMMENT ON COLUMN app.assignment_participants.cancelled_at IS 'Время отмены участия';
COMMENT ON COLUMN app.assignment_participants.cancel_reason IS 'Причина отмены участия: assignment_cancelled, removed_by_manager, left_group, video_deleted, group_deleted';

CREATE INDEX idx_assignment_participants_user_id_status ON app.assignment_participants(user_id, status);
CREATE INDEX idx_assignment_participants_status_due_at ON app.assignment_participants(status, due_at);
CREATE INDEX idx_assignment_participants_active_user_id ON app.assignment_participants(user_id)
    WHERE status IN ('assigned', 'in_progress');

CREATE TABLE app.assignment_events(
    event_id bigserial primary key,
    assignment_id uuid not null
        constraint fk_assignment_events_assignment_id references app.assignments(assignment_id) on delete cascade,
    user_id uuid null,
    type varchar not null,
    actor_id uuid null,
    payload jsonb not null default '{}',
    created_at timestamp not null default now()
);

COMMENT ON TABLE app.assignment_events IS 'Журнал событий назначения (аудит)';

COMMENT ON COLUMN app.assignment_events.event_id IS 'Идентификатор события, задаёт порядок';
COMMENT ON COLUMN app.assignment_events.assignment_id IS 'Назначение, к которому относится событие';
COMMENT ON COLUMN app.assignment_events.user_id IS 'Участник, к которому относится событие (NULL — событие относится к назначению целиком)';
COMMENT ON COLUMN app.assignment_events.type IS 'Тип события: created, due_changed, cancelled, participant_enrolled, participant_cancelled, participant_completed, participant_rejected';
COMMENT ON COLUMN app.assignment_events.actor_id IS 'Инициатор события (NULL — система: heartbeat или каскад)';
COMMENT ON COLUMN app.assignment_events.payload IS 'Детали события (например, старый/новый срок, причина, процент покрытия)';
COMMENT ON COLUMN app.assignment_events.created_at IS 'Время события';

CREATE INDEX idx_assignment_events_assignment_id_event_id ON app.assignment_events(assignment_id, event_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.assignment_events;
DROP TABLE IF EXISTS app.assignment_participants;
DROP TABLE IF EXISTS app.assignment_targets;
DROP TABLE IF EXISTS app.assignments;
-- +goose StatementEnd
