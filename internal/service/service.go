package service

import (
	"context"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/s3"

	"github.com/google/uuid"

	events "github.com/execaus/vilib-events"
)

//go:generate minimock -i Auth -o ./service_mocks/auth_mock.go
//go:generate minimock -i Account -o ./service_mocks/account_mock.go
//go:generate minimock -i AccountRole -o ./service_mocks/account_role_mock.go
//go:generate minimock -i User -o ./service_mocks/user_mock.go
//go:generate minimock -i Email -o ./service_mocks/email_mock.go
//go:generate minimock -i UserGroup -o ./service_mocks/user_group_mock.go
//go:generate minimock -i GroupMember -o ./service_mocks/group_member_mock.go
//go:generate minimock -i GroupRole -o ./service_mocks/group_role_mock.go
//go:generate minimock -i Video -o ./service_mocks/video_mock.go
//go:generate minimock -i VideoAsset -o ./service_mocks/video_asset_mock.go
//go:generate minimock -i Access -o ./service_mocks/access_mock.go
//go:generate minimock -i Outbox -o ./service_mocks/outbox_mock.go
//go:generate minimock -i Profile -o ./service_mocks/profile_mock.go
//go:generate minimock -i WatchProgress -o ./service_mocks/watch_progress_mock.go
//go:generate minimock -i Assignment -o ./service_mocks/assignment_mock.go
//go:generate minimock -i Chapter -o ./service_mocks/chapter_mock.go
//go:generate minimock -i vilib-api/internal/s3.S3 -o ./service_mocks/s3_mock.go

type Auth interface {
	GenerateToken(userID uuid.UUID, accounts []uuid.UUID, currentAccountID uuid.UUID) (string, error)
	ComparePassword(hashedPassword string, password string) bool
	HashPassword(password string) (string, error)
	GeneratePassword() (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetClaimsFromToken(token string) (*domain.AuthClaims, error)
	// IssueHLSToken выпускает короткоживущий JWT-токен доступа к HLS-плейлистам видео
	// (claims purpose/video_id/exp, §4.2 дизайна эпика).
	IssueHLSToken(videoID uuid.UUID, ttl time.Duration) (string, error)
	// ParseHLSToken проверяет подпись, срок действия и purpose HLS-токена. Любая ошибка
	// проверки возвращается как ErrUnauthorized (HTTP 401) — принадлежность токена конкретному
	// видео проверяет вызывающий сервис отдельно (HTTP 403 при несовпадении).
	ParseHLSToken(token string) (domain.HLSClaims, error)
	// SwitchAccount выпускает новый токен с current_account_id, переключённым на accountID —
	// организацию, в которой у пользователя (по email текущей строки) есть активная строка
	// (§2.4 дизайна эпика Э2). Нет строки в организации → ErrNotAccountMember (403 forbidden),
	// строка деактивирована → ErrForbiddenUserDeactivated (403 forbidden.user_deactivated).
	SwitchAccount(ctx context.Context, userID, accountID uuid.UUID) (string, error)
	// ChangePassword меняет пароль текущей строки пользователя userID (§6 дизайна эпика Э2,
	// поправка О-1: пароль — свойство организации, а не человека). Неверный oldPassword →
	// ErrOldPasswordInvalid (400 validation.old_password); newPassword совпадает со старым или
	// короче PasswordMinLength → ErrPasswordInvalid (400 validation.password).
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
	// RequestPasswordReset запрашивает сброс пароля по email (§6 дизайна эпика Э2, поправка
	// О-1). accountID задан — токен выдаётся только для этой строки (при её отсутствии или
	// неактивности — тихо ничего не делает); не задан и активная строка одна — она; несколько —
	// одно письмо со списком организаций и отдельным токеном на каждую. Email не найден или
	// строк нет — тихо ничего не делает (лог Warn). Ответ вызывающей стороне всегда успешен,
	// кроме ошибки отправки письма.
	RequestPasswordReset(ctx context.Context, email string, accountID *uuid.UUID) error
	// ResetPassword обновляет пароль строки пользователя, которой принадлежит токен
	// (§6 дизайна эпика Э2, поправка О-1). Токен не найден, использован или просрочен →
	// ErrResetTokenInvalid (400 validation.reset_token); newPassword короче PasswordMinLength →
	// ErrPasswordInvalid.
	ResetPassword(ctx context.Context, token, newPassword string) error
}

type Account interface {
	IsExistsUserByEmail(ctx context.Context, email string) (bool, error)
	GetByUserEmail(ctx context.Context, email string) ([]domain.Account, error)
	GetByID(ctx context.Context, accountsID ...uuid.UUID) ([]domain.Account, error)
	Create(ctx context.Context, userName, userSurname, email string) (domain.Account, error)
	CreateUser(ctx context.Context, accountID, initiatorID uuid.UUID, name, surname, email string) (domain.User, error)
	IsHasUser(ctx context.Context, accountID, initiatorID uuid.UUID) error
}

type AccountRole interface {
	Create(
		ctx context.Context,
		accountID, initiatorID uuid.UUID,
		name string, parentID *uuid.UUID, permission domain.PermissionMask, isDefault bool,
	) (domain.AccountRole, error)
	CreateSystemAccountOwner(ctx context.Context, accountID uuid.UUID) (domain.AccountRole, error)
	GetDefault(ctx context.Context, accountID uuid.UUID) (domain.AccountRole, error)
	GetByID(ctx context.Context, rolesID ...uuid.UUID) ([]domain.AccountRole, error)
	GetAll(ctx context.Context, initiatorID, accountID uuid.UUID) ([]domain.AccountRole, error)
	Delete(ctx context.Context, initiatorID, accountID, roleID uuid.UUID) error
	// Update — полная замена редактируемых полей роли аккаунта (§4 дизайна эпика Э2). Право
	// ManageRoles; системная роль — ErrIsSystemRole; бит AccountPermissionOwner в mask —
	// ErrPermissionOwnerForbidden; is_default=true снимает флаг у остальных ролей аккаунта
	// (ClearDefault) в той же транзакции; is_default=false у текущей дефолтной роли —
	// ErrDefaultRoleRequired; дубль имени — ErrAccountRoleNameExists.
	Update(
		ctx context.Context,
		initiatorID, accountID, roleID uuid.UUID,
		name string, parentID *uuid.UUID, mask domain.PermissionMask, isDefault bool,
	) (domain.AccountRole, error)
}

type User interface {
	Create(ctx context.Context, name, surname, email, password string, roleID uuid.UUID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) ([]domain.User, error)
	// Update частично обновляет пользователя accountID/targetID (§4 дизайна эпика Э2, «Блок
	// C»): смена роли (patch.RoleID != nil) или правка чужого профиля требуют ManageUsers;
	// инициатор, правящий собственное ФИО без смены роли (initiatorID == targetID &&
	// patch.RoleID == nil), — исключение без проверки прав. targetID должен состоять в
	// accountID (роль пользователя принадлежит accountID), иначе ErrNotFound. Все поля
	// patch nil — 200 без изменений.
	Update(ctx context.Context, initiatorID, accountID, targetID uuid.UUID, patch domain.UserPatch) (domain.User, error)
	GetByID(ctx context.Context, userID ...uuid.UUID) ([]domain.User, error)
	// GetByIDs батчем выбирает пользователей по списку идентификаторов (П-6 контракта Э2:
	// резолв авторов видео в списке видео). Отсутствие строки для части id — не ошибка.
	GetByIDs(ctx context.Context, userID []uuid.UUID) ([]domain.User, error)
	Deactivate(ctx context.Context, initiatorID, accountID, targetID uuid.UUID) error
	Reactivate(ctx context.Context, initiatorID, accountID, targetID uuid.UUID) error
	ListByAccount(
		ctx context.Context,
		initiatorID, accountID uuid.UUID,
		status repository.UserStatus,
	) ([]domain.User, error)
	// GetByEmailAndAccountID возвращает строку пользователя с указанным email в указанной
	// организации — используется переключением организации (§2.4 дизайна эпика Э2).
	GetByEmailAndAccountID(ctx context.Context, email string, accountID uuid.UUID) (domain.User, error)
	// UpdatePasswordHash обновляет хеш пароля одной строки пользователя (§6 дизайна эпика Э2,
	// поправка О-1). Строка не найдена — ErrNotFound.
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) (domain.User, error)
}

type Email interface {
	SendRegisteredMail(ctx context.Context, email, password string) error
	SendCreateUserEmail(ctx context.Context, email, password string) error
	// SendPasswordResetMail отправляет письмо со ссылкой (или списком ссылок по организациям)
	// сброса пароля (§6 дизайна эпика Э2, поправка О-1).
	SendPasswordResetMail(ctx context.Context, email string, links []domain.PasswordResetLink, ttl time.Duration) error
}

type UserGroup interface {
	Create(ctx context.Context, accountID, initiatorID uuid.UUID, name string) (domain.UserGroup, error)
	AddMembers(
		ctx context.Context,
		accountID, initiatorID, groupID uuid.UUID,
		targetsID ...uuid.UUID,
	) ([]domain.GroupMember, error)
	GetAll(ctx context.Context, initiatorID, accountID uuid.UUID) ([]domain.UserGroup, error)
	Delete(ctx context.Context, initiatorID, accountID, groupID uuid.UUID) error
	// GetByID выбирает группы по идентификаторам без проверки прав — батч-выборка для
	// внутренней сборки (например, профиля пользователя, §2.3 дизайна эпика Э2).
	GetByID(ctx context.Context, groupsID ...uuid.UUID) ([]domain.UserGroup, error)
	// Get возвращает карточку одной группы (§3.2 дизайна эпика Э2, П-3): право — любой
	// участник аккаунта (IsHasUser, как список групп); группа не в аккаунте — ErrNotFound.
	Get(ctx context.Context, initiatorID, accountID, groupID uuid.UUID) (domain.UserGroup, error)
	// Rename переименовывает группу (§4 дизайна эпика Э2, «Блок C»). Право — ManageGroups;
	// группа не в accountID — ErrNotFound; дубль имени в пределах аккаунта —
	// ErrGroupNameExists (409 conflict.group_name).
	Rename(ctx context.Context, initiatorID, accountID, groupID uuid.UUID, name string) (domain.UserGroup, error)
}

type GroupMember interface {
	Create(ctx context.Context, groupID, roleID uuid.UUID, usersID ...uuid.UUID) ([]domain.GroupMember, error)
	GetByUserIDAndGroupID(ctx context.Context, userID, groupID uuid.UUID) (domain.GroupMember, error)
	// RemoveMember удаляет участника groupID/targetID. Разрешено владельцу/участнику группы с
	// правом ManageMembers, либо (OR-логика, см. AddMembers) инициатору с правом уровня
	// аккаунта ManageGroups — в этом случае членство инициатора в группе не требуется.
	RemoveMember(ctx context.Context, accountID, initiatorID, groupID, targetID uuid.UUID) error
	// GetByUserID выбирает все членства пользователя во всех группах без проверки прав —
	// используется сборкой профиля пользователя (§2.3 дизайна эпика Э2).
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]domain.GroupMember, error)
	// ListByGroup возвращает участников группы вместе с данными пользователя и его роли в
	// группе (§3.2 дизайна эпика Э2, П-3). Право — IsCheckGroupAction(ManageGroups,
	// GroupPermissionManageMembers).
	ListByGroup(ctx context.Context, accountID, initiatorID, groupID uuid.UUID) ([]domain.GroupMemberDetails, error)
	// UpdateRole меняет роль участника группы (§3.3 дизайна эпика Э2, П-4). Право —
	// IsCheckGroupAction(ManageGroups, GroupPermissionManageMembers); роль должна принадлежать
	// accountID (иначе ErrNotFound); участник не найден (0 строк) — ErrNotFound.
	UpdateRole(
		ctx context.Context,
		accountID, initiatorID, groupID, userID, roleID uuid.UUID,
	) (domain.GroupMemberDetails, error)
}

type GroupRole interface {
	Create(
		ctx context.Context,
		accountID, initiatorID uuid.UUID,
		name string,
		mask domain.PermissionMask,
		isDefault bool,
	) (domain.GroupRole, error)
	GetByID(ctx context.Context, roleID uuid.UUID) ([]domain.GroupRole, error)
	GetDefault(ctx context.Context, accountID uuid.UUID) (domain.GroupRole, error)
	GetAll(ctx context.Context, initiatorID, accountID uuid.UUID) ([]domain.GroupRole, error)
	Delete(ctx context.Context, initiatorID, accountID, roleID uuid.UUID) error
	// GetByAccountID выбирает все роли групп аккаунта без проверки прав — батч-выборка для
	// сборки профиля пользователя (§2.3 дизайна эпика Э2): роль в собственных группах видна
	// пользователю без права ManageGroups, требуемого GetAll.
	GetByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.GroupRole, error)
	// Update — полная замена редактируемых полей роли группы (§4 дизайна эпика Э2). Право
	// ManageGroups; бит GroupPermissionOwner в mask разрешён (в отличие от ролей аккаунта);
	// is_default=true снимает флаг у остальных ролей групп аккаунта (ClearDefault) в той же
	// транзакции; is_default=false у текущей дефолтной роли — ErrDefaultRoleRequired; дубль
	// имени — ErrGroupRoleNameExists.
	Update(
		ctx context.Context,
		initiatorID, accountID, roleID uuid.UUID,
		name string, mask domain.PermissionMask, isDefault bool,
	) (domain.GroupRole, error)
}

type Video interface {
	// CreateUpload проверяет права ManageVideo, валидирует content-type/размер, создаёт
	// запись видео в статусе uploading и выдаёт преподписанный URL на PUT-загрузку оригинала.
	// isUrgent помечает видео срочным (эпик Э5, В-2) — публикуется в приоритетную полосу
	// обработки при подтверждении загрузки.
	CreateUpload(
		ctx context.Context,
		accountID, groupID, userID uuid.UUID,
		name, contentType string,
		size int64,
		isUrgent bool,
	) (domain.VideoUpload, error)
	// CompleteUpload подтверждает загрузку оригинала: проверяет объект в хранилище,
	// регистрирует ассет-оригинал, переводит видео в очередь на обработку и публикует
	// событие OriginalUploaded через outbox. Идемпотентна для видео, уже поставленных
	// в очередь/обрабатываемых/готовых. Возвращает карточку того же вида, что и элемент
	// списка видео — профили и автор объектом (§5.1 контракта Э2, П-6).
	CompleteUpload(
		ctx context.Context,
		accountID, groupID, userID, videoID uuid.UUID,
	) (domain.VideoListItem, error)
	// Get выбирает точку доступа к видео по статусу видео и флагу isPreferOriginal (§4.4
	// дизайна эпика): готовое видео без предпочтения оригинала — HLS-токен на мастер-плейлист,
	// иначе — преподписанный URL на оригинал. Возвращает ConflictError, если ни один из
	// вариантов недоступен (uploading или failed без загруженного оригинала).
	Get(
		ctx context.Context,
		accountID, groupID, initiatorID, videoID uuid.UUID,
		isPreferOriginal bool,
	) (domain.VideoAccess, error)
	// GetHLSMaster проверяет HLS-токен и отдаёт мастер-плейлист видео с URI вариантов,
	// переписанными на относительные ссылки с тем же токеном (§4.2, §4.3 дизайна эпика).
	GetHLSMaster(ctx context.Context, videoID uuid.UUID, token string) ([]byte, error)
	// GetHLSPlaylist проверяет HLS-токен и отдаёт медиаплейлист профиля с сегментами,
	// переписанными на преподписанные URL хранилища (§4.2, §4.3 дизайна эпика).
	GetHLSPlaylist(ctx context.Context, videoID uuid.UUID, profile domain.VideoProfile, token string) ([]byte, error)
	// GetAll возвращает список видео группы с профилями и признаком обработки, вычисленными
	// из ассетов (Э1-Т20). Причина сбоя (Failure) заполняется только для инициатора с правом
	// ManageVideo (аккаунтным или групповым) — иначе остаётся nil даже у видео в статусе
	// failed (Э1-Т17).
	GetAll(ctx context.Context, accountID, groupID, initiatorID uuid.UUID) ([]domain.VideoListItem, error)
	// Rename переименовывает видео и возвращает карточку того же вида, что и элемент списка
	// видео — профили и автор объектом (§5.1 контракта Э2, П-6).
	Rename(
		ctx context.Context,
		accountID, groupID, initiatorID, videoID uuid.UUID,
		name string,
	) (domain.VideoListItem, error)
	// Delete проверяет права ManageVideo, удаляет видео в БД и регистрирует best-effort
	// зачистку его объектов в хранилище после коммита транзакции (Э1-Т21, §7.3 дизайна эпика).
	Delete(ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID) error
	// DeleteObjectsAfterCommit регистрирует best-effort зачистку объектов перечисленных видео
	// в хранилище после коммита текущей транзакции саги (§7.3 дизайна эпика). Системный метод
	// без проверки прав — используется UserGroup.Delete для видео уже удалённой группы.
	DeleteObjectsAfterCommit(ctx context.Context, videoIDs ...uuid.UUID)
	// FailTimedOut переводит в failed(timeout) видео, зависшие в uploading/queued/compressing
	// дольше сконфигурированных таймаутов (§8 дизайна эпика, Э1-Т16). Вызывается watchdog'ом.
	FailTimedOut(ctx context.Context, now time.Time) (domain.TimedOutReport, error)
	// ApplyProcessingStarted переводит видео из очереди в обработку по событию ProcessingStarted
	// воркера. Системный вызов без проверки прав (аналог Update с initiatorID == nil); переход,
	// недопустимый для текущего статуса/номера попытки, игнорируется с логом (§7.2 эпика).
	ApplyProcessingStarted(ctx context.Context, evt events.Envelope, p events.ProcessingStarted) error
	// ApplyProcessingCompleted идемпотентно перерегистрирует ассеты результатов обработки и
	// переводит видео в готовность. Устаревшее событие или видео вне ожидаемого статуса —
	// best-effort зачистка результатов-сирот в хранилище после коммита транзакции (§7.2, §7.3).
	ApplyProcessingCompleted(ctx context.Context, evt events.Envelope, p events.ProcessingCompleted) error
	// ApplyProcessingFailed переводит видео в failed при постоянной ошибке или исчерпанных
	// попытках; при временной ошибке с запасом попыток возвращает видео в очередь и публикует
	// повторное событие OriginalUploaded с очередным номером попытки (§7.2 эпика).
	ApplyProcessingFailed(ctx context.Context, evt events.Envelope, p events.ProcessingFailed) error
}

type VideoAsset interface {
	Create(
		ctx context.Context,
		videoID uuid.UUID,
		kind domain.VideoAssetKind,
		profile domain.VideoProfile,
		bucket, key, contentType string,
		sizeBytes int64,
	) (domain.VideoAsset, error)
	Get(ctx context.Context, videoID uuid.UUID) ([]domain.VideoAsset, error)
	// SelectByVideoIDs выбирает ассеты сразу нескольких видео вместе с данными связанных файлов
	// (Э1-Т20, список видео группы). Пустой список идентификаторов не порождает запроса к БД.
	SelectByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) ([]domain.VideoAsset, error)
	// DeleteByVideoAndKinds удаляет ассеты видео указанных видов вместе со связанными файлами —
	// идемпотентная перерегистрация результатов обработки (Э1-Т14).
	DeleteByVideoAndKinds(ctx context.Context, videoID uuid.UUID, kinds []domain.VideoAssetKind) error
}

type Access interface {
	IsCheckAccountAction(
		ctx context.Context,
		accountID, initiatorID uuid.UUID, action domain.PermissionFlag,
	) error
	// IsCheckGroupAction реализует общую OR-логику доступа к операциям над группой (§3.1
	// дизайна эпика Э2): группа должна принадлежать accountID (иначе ErrNotFound); право
	// уровня аккаунта accountAction (в т.ч. владелец) разрешает без проверки членства в
	// группе; иначе инициатор должен состоять в группе и иметь Owner/groupAction в маске
	// своей групповой роли — отсутствие членства даёт ErrForbidden, а не 500.
	IsCheckGroupAction(
		ctx context.Context,
		accountID, initiatorID, groupID uuid.UUID,
		accountAction, groupAction domain.PermissionFlag,
	) error
	// CanManageAssignments проверяет право инициатора назначать обучение в области groupID —
	// группе видео назначения (§2 дизайна эпика Э3, решение В-3): аккаунтное
	// ManageAssignments (в т.ч. Owner) или групповое ManageAssignments (в т.ч. Owner группы)
	// при членстве в группе.
	CanManageAssignments(ctx context.Context, accountID, initiatorID, groupID uuid.UUID) error
	// CanWatchVideo определяет, может ли userID смотреть видео группы groupID (§2 дизайна
	// эпика Э3): аккаунтное VideoWatch/ManageVideo ИЛИ, при членстве, групповое
	// VideoWatch/ManageVideo/Owner. В отличие от IsCheck*Action не возвращает ошибку —
	// используется там, где отсутствие доступа не единственная причина отказа (В-4, отчёты).
	CanWatchVideo(ctx context.Context, accountID, userID, groupID uuid.UUID) bool
	// CanManageVideo проверяет право инициатора управлять видео группы groupID — переименование,
	// удаление, разметка глав (§2 дизайна эпика Э4): аккаунтное или групповое ManageVideo (в
	// т.ч. Owner). Парный метод CanWatchVideo, но, в отличие от него, возвращает ошибку — на
	// операциях изменения отсутствие права всегда единственная причина отказа.
	CanManageVideo(ctx context.Context, accountID, groupID, initiatorID uuid.UUID) error
	// ManagedAssignmentGroups определяет область чтения отчётов по назначениям инициатора
	// (В-8): all=true — аккаунтное право ManageAssignments/Owner (видны все назначения
	// аккаунта); иначе — список групп, где у инициатора Owner/ManageAssignments в групповой
	// роли (плюс собственные назначения по created_by — проверяется вызывающим сервисом).
	ManagedAssignmentGroups(ctx context.Context, accountID, initiatorID uuid.UUID) (bool, []uuid.UUID, error)
}

// Outbox публикует события в очередь Kafka внутри текущей транзакции саги (transactional
// outbox, §7.1 эпика) — обёртка над repository.Outbox.Insert.
type Outbox interface {
	Publish(ctx context.Context, topic, key string, payload []byte) error
}

// WatchProgress — прогресс просмотра видео пользователем: приём heartbeat'ов плеера, чтение
// текущего состояния и досчёт зачёта после появления длительности видео (§3 дизайна эпика Э3).
type WatchProgress interface {
	// Heartbeat принимает отрезок непрерывного воспроизведения от плеера и обновляет прогресс
	// просмотра инициатора по видео (шаги 1–8 алгоритма зачёта, §3 дизайна эпика).
	Heartbeat(
		ctx context.Context,
		accountID, groupID, initiatorID, videoID uuid.UUID,
		in domain.Heartbeat,
	) (domain.WatchState, error)
	// Get возвращает текущее состояние прогресса просмотра инициатора по видео без изменений
	// (нет строки прогресса — нулевое состояние).
	Get(ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID) (domain.WatchState, error)
	// OnDurationKnown досчитывает зачёт для пользователей, чей прогресс уже достиг порога до
	// того, как стала известна длительность видео (§3, «Э3-Т6») — вызывается из
	// VideoService.ApplyProcessingCompleted при переходе видео в ready.
	OnDurationKnown(ctx context.Context, videoID uuid.UUID, durationMs int64) error
	// CleanupStaleSessions удаляет сессии просмотра старше срока хранения (решение О-2 эпика
	// Э3) — вызывается тиком watchdog'а. Возвращает число удалённых строк.
	CleanupStaleSessions(ctx context.Context, now time.Time) (int64, error)
}

// Assignment реализует сервис назначений обязательного обучения — создание, чтение карточки
// и «мои назначения» (§4 дизайна эпика Э3).
type Assignment interface {
	// Create создаёт назначение видео пользователям и/или группе (§4 дизайна эпика Э3, шаги
	// 1–9): rejected — цели, не включённые в назначение (В-4: not_in_account/inactive/
	// no_access), не ошибка.
	Create(
		ctx context.Context, accountID, initiatorID uuid.UUID, in domain.CreateAssignment,
	) (domain.AssignmentDetails, []domain.RejectedTarget, error)
	// Get собирает карточку назначения целиком: цели, счётчики, участников с покрытием и
	// признаком доступа, журнал. Право чтения — В-8.
	Get(ctx context.Context, accountID, initiatorID, id uuid.UUID) (domain.AssignmentDetails, error)
	// ListMine собирает «мои назначения» пользователя во всех статусах (§4 дизайна эпика Э3).
	ListMine(ctx context.Context, userID uuid.UUID) ([]domain.MyAssignment, error)
	// List собирает список/отчёт по назначениям с фильтрами (§4, §5, §6 дизайна эпика Э3,
	// В-53): область — правило В-8 через Access.ManagedAssignmentGroups, счётчики — одним
	// батчем на все назначения, участники — только при f.ExpandParticipants.
	List(
		ctx context.Context,
		accountID, initiatorID uuid.UUID,
		f domain.AssignmentFilter,
	) ([]domain.AssignmentListItem, error)
	// ListForUser собирает отчёт по всем назначениям одного сотрудника (§4, §6 дизайна эпика
	// Э3, В-53) с учётом области В-8. userID должен состоять в accountID — иначе ErrNotFound.
	ListForUser(ctx context.Context, accountID, initiatorID, userID uuid.UUID) ([]domain.UserAssignmentItem, error)
	// UpdateDue меняет срок и/или комментарий назначения с пересчётом персональных сроков
	// незавершённых участников. Отменённое назначение не редактируется.
	UpdateDue(
		ctx context.Context, accountID, initiatorID, id uuid.UUID, patch domain.UpdateAssignment,
	) (domain.AssignmentDetails, error)
	// Cancel отменяет назначение целиком вместе с незавершёнными участиями. Повторная отмена
	// — ErrAssignmentCancelled.
	Cancel(ctx context.Context, accountID, initiatorID, id uuid.UUID) error
	// RemoveParticipant снимает участника с назначения; завершившего обучение снять нельзя
	// (ErrParticipantCompleted).
	RemoveParticipant(ctx context.Context, accountID, initiatorID, id, userID uuid.UUID) error
	// OnMembersAdded — системный каскад: зачисляет новых участников группы в действующие
	// назначения этой группы (Э3-Т3, правило срока В-5).
	OnMembersAdded(ctx context.Context, groupID uuid.UUID, userIDs []uuid.UUID) error
	// OnMemberRemoved — системный каскад: отменяет участия исключённого из группы сотрудника,
	// полученные через эту группу (Э3-Т30).
	OnMemberRemoved(ctx context.Context, groupID, userID uuid.UUID) error
	// OnVideoDeleted — системный каскад: отменяет действующие назначения удаляемого видео
	// (Э3-Т28).
	OnVideoDeleted(ctx context.Context, videoID uuid.UUID) error
	// OnGroupDeleted — системный каскад: отменяет действующие назначения видео удаляемой
	// группы (Э3-Т31).
	OnGroupDeleted(ctx context.Context, groupID uuid.UUID) error
}

// Chapter — CRUD глав видео и выдача глав с покрытием просмотра (§4 дизайна эпика Э4).
type Chapter interface {
	// List возвращает главы видео с покрытием инициатора, упорядоченные по времени начала.
	// Право — Access.CanWatchVideo (включает ManageVideo в OR). Видео без глав — пустой
	// список, не ошибка (Э4-Т4).
	List(
		ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID,
	) ([]domain.ChapterProgress, error)
	// Create создаёт главу видео. Право — Access.CanManageVideo. Видео должно быть в статусе
	// ready, start_ms — в пределах [0, duration_ms), не более 100 глав на видео, начало
	// уникально в пределах видео.
	Create(
		ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID, in domain.CreateChapter,
	) (domain.ChapterBound, error)
	// Update меняет начало и/или название главы. Право — Access.CanManageVideo. Диапазон
	// начала и готовность видео проверяются только при изменении StartMs.
	Update(
		ctx context.Context,
		accountID, groupID, initiatorID, videoID, chapterID uuid.UUID,
		patch domain.ChapterPatch,
	) (domain.ChapterBound, error)
	// Delete удаляет главу видео. Право — Access.CanManageVideo.
	Delete(ctx context.Context, accountID, groupID, initiatorID, videoID, chapterID uuid.UUID) error
}

// Profile агрегирует контекст текущего пользователя для ручки GET /me (§2.3 дизайна эпика Э2).
type Profile interface {
	// Get собирает профиль пользователя userID: организацию текущей строки, все организации
	// по email с активной строкой, роль в текущей организации, признак владельца аккаунта
	// и членства в группах. Деактивированная строка — ErrForbiddenUserDeactivated
	// (403 forbidden.user_deactivated).
	Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
}

type Service struct {
	Auth
	Account
	User
	Email
	AccountRole
	UserGroup
	GroupMember
	GroupRole
	Video
	VideoAsset
	Access
	Outbox
	Profile
	WatchProgress
	Assignment
	Chapter
}

func NewService(cfg config.Config, localMailBox chan string, s3 s3.S3, r *repository.Repository) *Service {
	s := &Service{}

	s.Auth = NewAuthService(cfg.Auth, cfg.Frontend, r.PasswordResetToken, s)
	s.Account = NewAccountService(r.Account, s)
	s.User = NewUserService(r.User, s)
	s.Email = NewEmailService(cfg.Email, cfg.Server.Mode, localMailBox)
	s.AccountRole = NewAccountRoleService(r.AccountRole, s)
	s.UserGroup = NewUserGroupService(r.UserGroup, s)
	s.GroupMember = NewGroupMemberService(r.GroupMember, s)
	s.GroupRole = NewGroupRoleService(r.GroupRole, s)
	s.Video = NewVideoService(s3, r.Video, s, VideoServiceConfig{
		Bucket:                      cfg.S3.Bucket,
		Video:                       cfg.Video,
		TopicOriginalUploaded:       cfg.Kafka.TopicOriginalUploaded,
		TopicOriginalUploadedUrgent: cfg.Kafka.TopicOriginalUploadedUrgent,
	}, WithPipelineProgress(r.PipelineProgress))
	s.VideoAsset = NewVideoAssetService(r.VideoAsset, s)
	s.Access = NewAccessService(s)
	s.Outbox = NewOutboxService(r.Outbox, s)
	s.Profile = NewProfileService(s)
	s.WatchProgress = NewWatchProgressService(
		r.WatchProgress,
		r.WatchSession,
		r.AssignmentParticipant,
		r.Video,
		s,
		cfg.Video,
	)
	s.Assignment = NewAssignmentService(
		r.Assignment,
		r.AssignmentTarget,
		r.AssignmentParticipant,
		r.AssignmentEvent,
		r.WatchProgress,
		r.Video,
		r.GroupMember,
		r.Chapter,
		s,
		cfg.Video,
	)
	s.Chapter = NewChapterService(r.Chapter, r.Video, s, cfg.Video)

	return s
}
