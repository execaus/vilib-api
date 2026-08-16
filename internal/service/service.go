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
}

type User interface {
	Create(ctx context.Context, name, surname, email, password string, roleID uuid.UUID) (domain.User, error)
	GetByEmail(ctx context.Context, email string) ([]domain.User, error)
	Update(ctx context.Context, initiatorID, accountID, targetID uuid.UUID, roleID *uuid.UUID) (domain.User, error)
	GetByID(ctx context.Context, userID ...uuid.UUID) ([]domain.User, error)
	Deactivate(ctx context.Context, initiatorID, accountID, targetID uuid.UUID) error
	Reactivate(ctx context.Context, initiatorID, accountID, targetID uuid.UUID) error
	ListByAccount(
		ctx context.Context,
		initiatorID, accountID uuid.UUID,
		status repository.UserStatus,
	) ([]domain.User, error)
}

type Email interface {
	SendRegisteredMail(ctx context.Context, email, password string) error
	SendCreateUserEmail(ctx context.Context, email, password string) error
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
}

type GroupMember interface {
	Create(ctx context.Context, groupID, roleID uuid.UUID, usersID ...uuid.UUID) ([]domain.GroupMember, error)
	GetByUserIDAndGroupID(ctx context.Context, userID, groupID uuid.UUID) (domain.GroupMember, error)
	// RemoveMember удаляет участника groupID/targetID. Разрешено владельцу/участнику группы с
	// правом ManageMembers, либо (OR-логика, см. AddMembers) инициатору с правом уровня
	// аккаунта ManageGroups — в этом случае членство инициатора в группе не требуется.
	RemoveMember(ctx context.Context, accountID, initiatorID, groupID, targetID uuid.UUID) error
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
}

type Video interface {
	// CreateUpload проверяет права ManageVideo, валидирует content-type/размер, создаёт
	// запись видео в статусе uploading и выдаёт преподписанный URL на PUT-загрузку оригинала.
	CreateUpload(
		ctx context.Context,
		accountID, groupID, userID uuid.UUID,
		name, contentType string,
		size int64,
	) (domain.VideoUpload, error)
	// CompleteUpload подтверждает загрузку оригинала: проверяет объект в хранилище,
	// регистрирует ассет-оригинал, переводит видео в очередь на обработку и публикует
	// событие OriginalUploaded через outbox. Идемпотентна для видео, уже поставленных
	// в очередь/обрабатываемых/готовых.
	CompleteUpload(ctx context.Context, accountID, groupID, userID, videoID uuid.UUID) (domain.Video, error)
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
	Rename(ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID, name string) (domain.Video, error)
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
}

// Outbox публикует события в очередь Kafka внутри текущей транзакции саги (transactional
// outbox, §7.1 эпика) — обёртка над repository.Outbox.Insert.
type Outbox interface {
	Publish(ctx context.Context, topic, key string, payload []byte) error
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
}

func NewService(cfg config.Config, localMailBox chan string, s3 s3.S3, r *repository.Repository) *Service {
	s := &Service{}

	s.Auth = NewAuthService(cfg.Auth, s)
	s.Account = NewAccountService(r.Account, s)
	s.User = NewUserService(r.User, s)
	s.Email = NewEmailService(cfg.Email, cfg.Server.Mode, localMailBox)
	s.AccountRole = NewAccountRoleService(r.AccountRole, s)
	s.UserGroup = NewUserGroupService(r.UserGroup, s)
	s.GroupMember = NewGroupMemberService(r.GroupMember, s)
	s.GroupRole = NewGroupRoleService(r.GroupRole, s)
	s.Video = NewVideoService(s3, r.Video, s, VideoServiceConfig{
		Bucket:                cfg.S3.Bucket,
		Video:                 cfg.Video,
		TopicOriginalUploaded: cfg.Kafka.TopicOriginalUploaded,
	})
	s.VideoAsset = NewVideoAssetService(r.VideoAsset, s)
	s.Access = NewAccessService(s)
	s.Outbox = NewOutboxService(r.Outbox, s)

	return s
}
