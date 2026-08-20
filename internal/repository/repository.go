package repository

import (
	"context"
	"encoding/json"
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

//go:generate minimock -i Account -o ./repository_mocks/account_mock.go
//go:generate minimock -i User -o ./repository_mocks/user_mock.go
//go:generate minimock -i AccountRole -o ./repository_mocks/account_role_mock.go
//go:generate minimock -i UserGroup -o ./repository_mocks/user_group_mock.go
//go:generate minimock -i GroupMember -o ./repository_mocks/group_member_mock.go
//go:generate minimock -i GroupRole -o ./repository_mocks/group_role_mock.go
//go:generate minimock -i Video -o ./repository_mocks/video_mock.go
//go:generate minimock -i VideoAsset -o ./repository_mocks/video_asset_mock.go
//go:generate minimock -i Outbox -o ./repository_mocks/outbox_mock.go
//go:generate minimock -i PasswordResetToken -o ./repository_mocks/password_reset_token_mock.go
//go:generate minimock -i WatchProgress -o ./repository_mocks/watch_progress_mock.go
//go:generate minimock -i WatchSession -o ./repository_mocks/watch_session_mock.go
//go:generate minimock -i Assignment -o ./repository_mocks/assignment_mock.go
//go:generate minimock -i AssignmentTarget -o ./repository_mocks/assignment_target_mock.go
//go:generate minimock -i AssignmentParticipant -o ./repository_mocks/assignment_participant_mock.go
//go:generate minimock -i AssignmentEvent -o ./repository_mocks/assignment_event_mock.go

// UserStatus определяет фильтр по активности пользователей при выборке.
type UserStatus string

const (
	UserStatusActive      UserStatus = "active"
	UserStatusDeactivated UserStatus = "deactivated"
	UserStatusAll         UserStatus = "all"
)

type Account interface {
	Insert(ctx context.Context, name, email string) (domain.Account, error)
	SelectByUsersID(ctx context.Context, usersID ...uuid.UUID) ([]domain.Account, error)
	SelectByID(ctx context.Context, accountsID ...uuid.UUID) ([]domain.Account, error)
}

type User interface {
	SelectByEmail(ctx context.Context, email string) ([]domain.User, error)
	Insert(ctx context.Context, name, surname, hash, email string, roleID uuid.UUID) (domain.User, error)
	SelectByID(ctx context.Context, usersID ...uuid.UUID) ([]domain.User, error)
	// SelectByIDs батчем выбирает пользователей по списку идентификаторов одним запросом
	// (П-6 контракта Э2: резолв авторов видео в списке). Отсутствие строки для части id —
	// не ошибка, такие идентификаторы просто не попадают в результат.
	SelectByIDs(ctx context.Context, usersID []uuid.UUID) ([]domain.User, error)
	UpdateRole(ctx context.Context, userID, roleID uuid.UUID) (domain.User, error)
	// UpdateProfile частично обновляет ФИО пользователя (§4 дизайна эпика Э2, «Блок C»): nil-
	// поле оставляет значение без изменений. Оба поля nil — строка не трогается, возвращается
	// текущее состояние пользователя. Пользователь не найден — ErrNotFound.
	UpdateProfile(ctx context.Context, userID uuid.UUID, name, surname *string) (domain.User, error)
	Deactivate(ctx context.Context, userID uuid.UUID) error
	Reactivate(ctx context.Context, userID uuid.UUID) error
	SelectByAccountID(ctx context.Context, accountID uuid.UUID, status UserStatus) ([]domain.User, error)
	// SelectByEmailAndAccountID выбирает строку пользователя с указанным email в указанной
	// организации — используется переключением организации (§2.4 дизайна эпика Э2).
	SelectByEmailAndAccountID(ctx context.Context, email string, accountID uuid.UUID) (domain.User, error)
	// UpdatePasswordHash обновляет хеш пароля одной строки пользователя (§6 дизайна эпика Э2,
	// поправка О-1: пароль — свойство организации, а не человека). Строка не найдена —
	// ErrNotFound.
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) (domain.User, error)
}

type AccountRole interface {
	Insert(
		ctx context.Context,
		accountID uuid.UUID, name string, parentID *uuid.UUID, permission domain.PermissionMask, isDefault, isSystem bool,
	) (domain.AccountRole, error)
	SelectByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.AccountRole, error)
	SelectByID(ctx context.Context, rolesID ...uuid.UUID) ([]domain.AccountRole, error)
	Delete(ctx context.Context, roleID uuid.UUID) error
	SelectActiveUsersByRole(ctx context.Context, roleID uuid.UUID) ([]domain.User, error)
	ResetRoleToDefault(ctx context.Context, oldRoleID, defaultRoleID uuid.UUID) error
	// Update заменяет все редактируемые поля роли аккаунта (полная замена, §4 дизайна эпика
	// Э2). isSystem и accountID не меняются.
	Update(
		ctx context.Context,
		roleID uuid.UUID, name string, parentID *uuid.UUID, permission domain.PermissionMask, isDefault bool,
	) (domain.AccountRole, error)
	// ClearDefault снимает флаг is_default со всех ролей аккаунта — вызывается перед Update/
	// Insert с isDefault=true, чтобы в аккаунте всегда была не больше одной роли по умолчанию.
	ClearDefault(ctx context.Context, accountID uuid.UUID) error
}

type UserGroup interface {
	Insert(ctx context.Context, accountID uuid.UUID, name string) (domain.UserGroup, error)
	GetByID(ctx context.Context, groupsID ...uuid.UUID) ([]domain.UserGroup, error)
	SelectByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.UserGroup, error)
	// UpdateName переименовывает группу (§4 дизайна эпика Э2, «Блок C»). Группа не найдена —
	// ErrNotFound; дубль имени в пределах аккаунта — dberrors.UserGroupErrors.
	// ErrUniqueUserGroupsNameAccountIdKey.
	UpdateName(ctx context.Context, groupID uuid.UUID, name string) (domain.UserGroup, error)
	// DeleteCascade удаляет группу вместе со всеми её видео, ассетами, файлами и участниками
	// (Э1-Т21). Возвращает идентификаторы удалённых видео группы — нужны вызывающей стороне
	// для best-effort зачистки их объектов в хранилище после коммита (§7.3 эпика).
	DeleteCascade(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error)
}

type GroupMember interface {
	Insert(ctx context.Context, groupID, roleID uuid.UUID, usersID ...uuid.UUID) ([]domain.GroupMember, error)
	SelectByUserIDAndGroupID(ctx context.Context, userID, groupID uuid.UUID) (domain.GroupMember, error)
	// SelectByUserID выбирает все членства пользователя во всех группах (агрегация профиля
	// GET /me, §2.3 дизайна эпика Э2). Отсутствие членств — пустой срез, не ошибка.
	SelectByUserID(ctx context.Context, userID uuid.UUID) ([]domain.GroupMember, error)
	// SelectByGroupID выбирает всех участников группы (§3.2 дизайна эпика Э2, П-3). Отсутствие
	// участников — пустой срез, не ошибка.
	SelectByGroupID(ctx context.Context, groupID uuid.UUID) ([]domain.GroupMember, error)
	Delete(ctx context.Context, groupID, userID uuid.UUID) error
	// UpdateRole меняет роль участника группы (§3.3 дизайна эпика Э2, П-4). Участник не
	// найден (0 строк) — ErrNotFound.
	UpdateRole(ctx context.Context, groupID, userID, roleID uuid.UUID) (domain.GroupMember, error)
}

type GroupRole interface {
	Insert(
		ctx context.Context,
		accountID uuid.UUID,
		name string,
		permission domain.PermissionMask,
		isDefault bool,
	) (domain.GroupRole, error)
	SelectByAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GroupRole, error)
	SelectByID(ctx context.Context, roleID uuid.UUID) ([]domain.GroupRole, error)
	GetDefault(ctx context.Context, groupID uuid.UUID) (domain.GroupRole, error)
	SelectMembersByRole(ctx context.Context, roleID uuid.UUID) ([]domain.GroupMember, error)
	Delete(ctx context.Context, roleID uuid.UUID) error
	// Update заменяет все редактируемые поля роли группы (полная замена, §4 дизайна эпика Э2).
	// accountID не меняется.
	Update(
		ctx context.Context,
		roleID uuid.UUID, name string, permission domain.PermissionMask, isDefault bool,
	) (domain.GroupRole, error)
	// ClearDefault снимает флаг is_default со всех ролей групп аккаунта — вызывается перед
	// Update/Insert с isDefault=true.
	ClearDefault(ctx context.Context, accountID uuid.UUID) error
}

type Video interface {
	Select(ctx context.Context, id uuid.UUID) (*domain.Video, error)
	Insert(ctx context.Context, name string, groupID, userID uuid.UUID, status domain.VideoStatus) (domain.Video, error)
	// UpdateStatusIf выполняет условный переход статуса видео: строка обновляется, только
	// если её текущий статус входит в from (и, если задан patch.ExpectedAttempt, совпадает
	// текущий processing_attempt). Возвращает true, если строка была обновлена — гонки
	// между watchdog'ом и обработчиком событий безопасны (§1.3 эпика).
	UpdateStatusIf(
		ctx context.Context,
		id uuid.UUID,
		from []domain.VideoStatus,
		to domain.VideoStatus,
		patch domain.VideoPatch,
	) (bool, error)
	SelectByGroupID(ctx context.Context, groupID uuid.UUID) ([]domain.Video, error)
	// SelectByIDs батчем выбирает видео по списку идентификаторов (§4 дизайна эпика Э3,
	// AssignmentService.ListMine). Отсутствие строки для части id — не ошибка.
	SelectByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Video, error)
	UpdateName(ctx context.Context, videoID uuid.UUID, name string) (domain.Video, error)
	// Delete удаляет видео вместе со всеми его ассетами и файлами (Э1-Т21) — без сирот в БД.
	Delete(ctx context.Context, videoID uuid.UUID) error
	// UpdateTimedOut переводит в failed(timeout) все видео заданного статуса, чья контрольная
	// временная метка (created_at для uploading, status_changed_at для остальных) старше
	// before — один атомарный условный UPDATE, возвращает id обновлённых строк (§8 дизайна
	// эпика).
	UpdateTimedOut(
		ctx context.Context,
		status domain.VideoStatus,
		before time.Time,
		failure domain.VideoFailure,
	) ([]uuid.UUID, error)
}

type VideoAsset interface {
	Select(ctx context.Context, videoID uuid.UUID) ([]domain.VideoAsset, error)
	// SelectByVideoIDs выбирает ассеты сразу нескольких видео вместе с данными связанных файлов
	// (Э1-Т20, список видео группы). Пустой список идентификаторов не порождает запроса к БД.
	SelectByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) ([]domain.VideoAsset, error)
	Insert(
		ctx context.Context,
		videoID uuid.UUID,
		kind domain.VideoAssetKind,
		profile domain.VideoProfile,
		bucket, key, contentType string,
		sizeBytes int64,
	) (domain.VideoAsset, error)
	// DeleteByVideoAndKinds удаляет ассеты видео указанных видов вместе со связанными файлами.
	// Используется для идемпотентной перерегистрации результатов обработки (Э1-Т14).
	DeleteByVideoAndKinds(ctx context.Context, videoID uuid.UUID, kinds []domain.VideoAssetKind) error
}

// PasswordResetToken — репозиторий одноразовых токенов сброса пароля (§6, §7 дизайна эпика Э2,
// поправка О-1). В базе хранится только SHA-256 хеш токена.
type PasswordResetToken interface {
	// Insert создаёт токен сброса пароля для строки пользователя userID в рамках текущей
	// транзакции саги.
	Insert(
		ctx context.Context,
		userID uuid.UUID, email, tokenHash string, expiresAt time.Time,
	) (domain.PasswordResetToken, error)
	// SelectByHash выбирает токен по хешу — строки не существует → ErrNotFound.
	SelectByHash(ctx context.Context, tokenHash string) (domain.PasswordResetToken, error)
	// MarkUsed помечает токен использованным (used_at = now).
	MarkUsed(ctx context.Context, tokenID uuid.UUID) error
	// DeleteByEmail удаляет все токены сброса пароля указанного email — вызывается перед
	// выдачей новых токенов и после успешного сброса пароля.
	DeleteByEmail(ctx context.Context, email string) error
}

// WatchProgress — репозиторий прогресса просмотра видео пользователем (§1.4, §3 дизайна
// эпика Э3): объединённые интервалы heartbeat'ов и накопленные метрики.
type WatchProgress interface {
	// SelectForUpdate выбирает и блокирует (FOR UPDATE) строку прогресса пользователя по
	// видео — первый шаг сериализации heartbeat'ов одной пары (user, video). Строка не
	// найдена — ErrNotFound.
	SelectForUpdate(ctx context.Context, userID, videoID uuid.UUID) (domain.WatchProgress, error)
	// InsertEmpty создаёт пустую строку прогресса (first_at/last_at = now) — идемпотентно,
	// ON CONFLICT по первичному ключу (user_id, video_id) ничего не делает. Вызывающая
	// сторона обязана перечитать строку через SelectForUpdate, чтобы получить блокировку.
	InsertEmpty(ctx context.Context, userID, videoID uuid.UUID, now time.Time) error
	// Apply — единственный UPDATE применения принятого интервала heartbeat'а (§3 шаг 6):
	// объединяет intervals оператором `+`, пересчитывает covered_ms без чтения-изменения в
	// Go, увеличивает wall_ms на wallDeltaMs, обновляет last_position_ms/last_at и
	// выставляет threshold_reached_at при первом достижении needMs (огромное значение —
	// сентинел «длительность видео ещё не известна»).
	Apply(
		ctx context.Context,
		userID, videoID uuid.UUID,
		fromMs, toMs, positionMs, wallDeltaMs int64,
		now time.Time, needMs int64,
	) (domain.WatchProgress, error)
	// UpdatePosition обновляет только позицию плеера и last_at — heartbeat, чей интервал не
	// был зачтён целиком (отброшен или урезан до нуля).
	UpdatePosition(
		ctx context.Context, userID, videoID uuid.UUID, positionMs int64, now time.Time,
	) (domain.WatchProgress, error)
	// OnDurationKnown выставляет threshold_reached_at всем строкам видео, чьё покрытие уже
	// достигло needMs, но порог ещё не был зафиксирован (§3, «Э3-Т6»: длительность видео
	// появилась позже накопленного прогресса) — возвращает id таких пользователей.
	OnDurationKnown(ctx context.Context, videoID uuid.UUID, needMs int64, now time.Time) ([]uuid.UUID, error)
	// SelectByUserAndVideoIDs батчем выбирает прогресс пользователя по списку видео.
	SelectByUserAndVideoIDs(ctx context.Context, userID uuid.UUID, videoIDs []uuid.UUID) ([]domain.WatchProgress, error)
	// SelectByVideoIDs батчем выбирает прогресс всех пользователей по списку видео (отчёты).
	SelectByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) ([]domain.WatchProgress, error)
}

// WatchSession — репозиторий сессий просмотра: идемпотентность heartbeat'ов и защита от
// перемотки в рамках одной сессии плеера (§1.5 дизайна эпика Э3).
type WatchSession interface {
	// SelectForUpdate выбирает и блокирует (FOR UPDATE) сессию по идентификатору — второй
	// шаг сериализации heartbeat'ов (после блокировки строки прогресса, порядок блокировок
	// фиксирован). Строка не найдена — ErrNotFound.
	SelectForUpdate(ctx context.Context, sessionID uuid.UUID) (domain.WatchSession, error)
	// Insert создаёт новую сессию просмотра — session_id генерирует клиент при открытии
	// плеера.
	Insert(
		ctx context.Context, sessionID, userID, videoID uuid.UUID, now time.Time, positionMs int64,
	) (domain.WatchSession, error)
	// Update продвигает сессию на очередной принятый heartbeat: last_seq, last_at,
	// last_position_ms.
	Update(
		ctx context.Context,
		sessionID uuid.UUID,
		seq int64,
		now time.Time,
		positionMs int64,
	) (domain.WatchSession, error)
	// DeleteOlderThan удаляет сессии, чей last_at старше cutoff — периодическая чистка
	// watchdog'ом (В-52). Возвращает число удалённых строк.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// Assignment — репозиторий назначений обязательного обучения (§1.1, §4 дизайна эпика Э3).
type Assignment interface {
	// Insert создаёт назначение в статусе active со снимком названия видео и группы
	// (Э3-Т7). dueAt/dueDays заполняется по dueMode.
	Insert(
		ctx context.Context,
		accountID, videoID uuid.UUID, videoName string,
		groupID uuid.UUID, groupName string,
		createdBy uuid.UUID,
		dueMode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int,
		comment string,
	) (domain.Assignment, error)
	// SelectByID выбирает назначение по идентификатору. Строка не найдена — ErrNotFound.
	SelectByID(ctx context.Context, id uuid.UUID) (domain.Assignment, error)
	// SelectByIDs батчем выбирает назначения по списку идентификаторов (AssignmentService.
	// ListMine). Отсутствие строки для части id — не ошибка.
	SelectByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Assignment, error)
	// UpdateDue меняет режим и значение срока назначения (AssignmentService.UpdateDue).
	// Заполняется поле режима, противоположное — обнуляется.
	UpdateDue(
		ctx context.Context,
		id uuid.UUID,
		dueMode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int,
	) (domain.Assignment, error)
	// UpdateComment меняет комментарий назначения; пустая строка очищает его.
	UpdateComment(ctx context.Context, id uuid.UUID, comment string) (domain.Assignment, error)
	// Cancel переводит назначение в статус cancelled. Условие status='active' в запросе
	// защищает от повторной отмены: уже отменённое назначение возвращает false.
	// cancelledBy — nil для системных отмен (удаление видео или группы).
	Cancel(
		ctx context.Context,
		id uuid.UUID, cancelledBy *uuid.UUID,
		reason domain.AssignmentCancelReason, at time.Time,
	) (bool, error)
	// SelectActiveByTargetGroup выбирает действующие назначения, адресованные группе как цели
	// (каскад OnMembersAdded — зачисление новых участников группы).
	SelectActiveByTargetGroup(ctx context.Context, groupID uuid.UUID) ([]domain.Assignment, error)
	// SelectActiveByVideoIDs выбирает действующие назначения перечисленных видео (каскад
	// OnVideoDeleted). Пустой список идентификаторов не порождает запроса к БД.
	SelectActiveByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) ([]domain.Assignment, error)
	// SelectActiveByGroupID выбирает действующие назначения видео указанной группы (каскад
	// OnGroupDeleted).
	SelectActiveByGroupID(ctx context.Context, groupID uuid.UUID) ([]domain.Assignment, error)
}

// AssignmentTarget — репозиторий целей назначения: конкретный пользователь или группа
// (§1.2 дизайна эпика Э3).
type AssignmentTarget interface {
	// InsertBatch создаёт строки целей назначения. Конфликтов при создании назначения не
	// бывает (assignment_id всегда новый).
	InsertBatch(ctx context.Context, targets []domain.AssignmentTarget) ([]domain.AssignmentTarget, error)
	// SelectByAssignmentIDs батчем выбирает цели нескольких назначений (карточка/список).
	SelectByAssignmentIDs(ctx context.Context, assignmentIDs []uuid.UUID) ([]domain.AssignmentTarget, error)
}

// AssignmentParticipant — репозиторий персональных записей участников назначений (§1.3, §4
// дизайна эпика Э3).
type AssignmentParticipant interface {
	// InsertBatch создаёт или реактивирует персональные записи участников назначения (семантика
	// «OnMembersAdded», §4 дизайна эпика Э3): конфликт по (assignment_id, user_id) обновляет
	// только отменённые записи, завершённые и активные неприкосновенны (Э3-Н1) — такие строки
	// молча пропускаются в результате.
	InsertBatch(
		ctx context.Context,
		participants []domain.AssignmentParticipant,
	) ([]domain.AssignmentParticipant, error)
	// SelectByAssignmentIDs батчем выбирает участников нескольких назначений (карточка/список).
	SelectByAssignmentIDs(ctx context.Context, assignmentIDs []uuid.UUID) ([]domain.AssignmentParticipant, error)
	// SelectByUserID выбирает все персональные записи пользователя во всех статусах
	// (AssignmentService.ListMine).
	SelectByUserID(ctx context.Context, userID uuid.UUID) ([]domain.AssignmentParticipant, error)
	// CountByAssignmentIDs агрегирует счётчики статусов участников по каждому назначению
	// (включая просроченных) одним запросом GROUP BY.
	CountByAssignmentIDs(
		ctx context.Context,
		assignmentIDs []uuid.UUID,
	) (map[uuid.UUID]domain.AssignmentCounters, error)
	// UpdateStatusByUserVideo переводит статус участника из from в to для всех активных
	// назначений видео videoID, в которых участвует userID. Возвращает id обновлённых
	// назначений.
	UpdateStatusByUserVideo(
		ctx context.Context, userID, videoID uuid.UUID,
		from, to domain.AssignmentParticipantStatus,
	) ([]uuid.UUID, error)
	// CompleteByUserVideo завершает участие userID во всех активных назначениях видео
	// videoID, ещё не завершённых (assigned/in_progress) — условие WHERE в самом запросе
	// гарантирует неизменяемость completed_* (Э3-Н1: повторный вызов не переписывает уже
	// завершённых участников). Возвращает id завершённых назначений.
	CompleteByUserVideo(
		ctx context.Context,
		userID, videoID uuid.UUID,
		completedAt time.Time, coveragePct, thresholdPct int,
		sessionID *uuid.UUID,
	) ([]uuid.UUID, error)
	// SelectByAssignmentIDAndUserID выбирает персональную запись участника назначения
	// (AssignmentService.RemoveParticipant). Строка не найдена — ErrNotFound.
	SelectByAssignmentIDAndUserID(
		ctx context.Context, assignmentID, userID uuid.UUID,
	) (domain.AssignmentParticipant, error)
	// UpdateDueByAssignment пересчитывает персональные сроки незавершённых участников
	// назначения: режим date ставит общий срок, режим days — enrolled_at + dueDays.
	// Завершённые и отменённые записи не затрагиваются (Э3-Н1). Возвращает id пользователей,
	// чей срок изменился.
	UpdateDueByAssignment(
		ctx context.Context,
		assignmentID uuid.UUID,
		dueMode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int,
	) ([]uuid.UUID, error)
	// CancelByAssignment отменяет незавершённых участников назначения (отмена назначения,
	// удаление видео или группы). Возвращает id отменённых пользователей.
	CancelByAssignment(
		ctx context.Context,
		assignmentID uuid.UUID,
		reason domain.AssignmentParticipantCancelReason, at time.Time,
	) ([]uuid.UUID, error)
	// CancelOne отменяет участие одного пользователя в назначении (снятие участника
	// менеджером). Завершённая запись не затрагивается — возвращает false.
	CancelOne(
		ctx context.Context,
		assignmentID, userID uuid.UUID,
		reason domain.AssignmentParticipantCancelReason, at time.Time,
	) (bool, error)
	// CancelBySourceGroupAndUser отменяет участия пользователя, полученные через членство в
	// группе (source=group), во всех действующих назначениях — каскад исключения из группы.
	// Личные назначения (source=personal) не затрагиваются (Э3-Т30). Возвращает id назначений,
	// в которых участие отменено.
	CancelBySourceGroupAndUser(
		ctx context.Context,
		groupID, userID uuid.UUID,
		reason domain.AssignmentParticipantCancelReason, at time.Time,
	) ([]uuid.UUID, error)
}

// AssignmentEvent — репозиторий журнала назначения (§1.6 дизайна эпика Э3, Э3-Т32).
type AssignmentEvent interface {
	// Insert создаёт одну запись журнала.
	Insert(
		ctx context.Context,
		assignmentID uuid.UUID, userID *uuid.UUID,
		eventType domain.AssignmentEventType, actorID *uuid.UUID,
		payload json.RawMessage, now time.Time,
	) (domain.AssignmentEvent, error)
	// InsertBatch создаёт несколько записей журнала в текущей транзакции.
	InsertBatch(ctx context.Context, events []domain.AssignmentEvent) ([]domain.AssignmentEvent, error)
	// SelectByAssignmentID выбирает журнал назначения в хронологическом порядке.
	SelectByAssignmentID(ctx context.Context, assignmentID uuid.UUID) ([]domain.AssignmentEvent, error)
}

// Outbox — репозиторий очереди исходящих событий Kafka (transactional outbox, §7.1 эпика).
type Outbox interface {
	// Insert кладёт событие в очередь публикации внутри текущей транзакции.
	Insert(ctx context.Context, topic, key string, payload []byte) error
	// SelectBatchForUpdate выбирает и блокирует (FOR UPDATE SKIP LOCKED) до limit старейших
	// событий очереди для публикации релеем.
	SelectBatchForUpdate(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	// DeleteByIDs удаляет опубликованные события.
	DeleteByIDs(ctx context.Context, ids []int64) error
}

type Repository struct {
	Account
	User
	AccountRole
	UserGroup
	GroupMember
	GroupRole
	Video
	VideoAsset
	Outbox
	PasswordResetToken
	WatchProgress
	WatchSession
	Assignment
	AssignmentTarget
	AssignmentParticipant
	AssignmentEvent
}

func NewRepository(provider *ExecutorProvider) *Repository {
	return &Repository{
		Account:               NewAccountRepository(provider),
		User:                  NewUserRepository(provider),
		AccountRole:           NewAccountRoleRepository(provider),
		UserGroup:             NewUserGroupRepository(provider),
		GroupMember:           NewGroupMemberRepository(provider),
		GroupRole:             NewGroupRoleRepository(provider),
		Video:                 NewVideoRepository(provider),
		VideoAsset:            NewVideoAssetRepository(provider),
		Outbox:                NewOutboxRepository(provider),
		PasswordResetToken:    NewPasswordResetTokenRepository(provider),
		WatchProgress:         NewWatchProgressRepository(provider),
		WatchSession:          NewWatchSessionRepository(provider),
		Assignment:            NewAssignmentRepository(provider),
		AssignmentTarget:      NewAssignmentTargetRepository(provider),
		AssignmentParticipant: NewAssignmentParticipantRepository(provider),
		AssignmentEvent:       NewAssignmentEventRepository(provider),
	}
}
