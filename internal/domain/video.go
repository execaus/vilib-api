package domain

import (
	"time"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type (
	PreflightURL string
	VideoStatus  uint
)

const (
	VideoUploadURLTTL = time.Hour
	VideoStreamURLTTL = time.Hour
)

const (
	VideoStatusUploading VideoStatus = iota
	VideoStatusCompressing
	VideoStatusReady
	VideoStatusFailed
	VideoStatusQueued
)

// String возвращает человекочитаемое имя статуса видео.
func (s VideoStatus) String() string {
	switch s {
	case VideoStatusUploading:
		return "uploading"
	case VideoStatusCompressing:
		return "compressing"
	case VideoStatusReady:
		return "ready"
	case VideoStatusFailed:
		return "failed"
	case VideoStatusQueued:
		return "queued"
	default:
		return "unknown"
	}
}

// VideoAssetKind определяет вид ассета видео.
type VideoAssetKind string

const (
	VideoAssetKindOriginal   VideoAssetKind = "original"
	VideoAssetKindHLSMaster  VideoAssetKind = "hls_master"
	VideoAssetKindHLSVariant VideoAssetKind = "hls_variant"
)

// VideoProfile — имя профиля качества (например, "360p"). Набор профилей конфигурируемый,
// поэтому фиксированного перечня констант нет.
type VideoProfile string

// VideoFailureClass определяет класс ошибки обработки видео.
type VideoFailureClass string

const (
	VideoFailureClassPermanent VideoFailureClass = "permanent"
	VideoFailureClassTemporary VideoFailureClass = "temporary"
	VideoFailureClassTimeout   VideoFailureClass = "timeout"
)

// VideoPrefix вычисляет префикс всех ключей объектов видео в хранилище (§3.3 эпика): оригинал
// и все результаты обработки лежат под одним префиксом videos/{video_id}/, поэтому удаление
// видео стирает всё одним DeleteByPrefix.
func VideoPrefix(videoID uuid.UUID) string {
	return "videos/" + videoID.String() + "/"
}

// VideoOriginalObjectKey вычисляет ключ объекта оригинала видео в хранилище (§3.3 эпика).
// Ключ выводится из одного video_id в любой точке (complete-ручка, watchdog, удаление) без
// хранения промежуточного состояния между созданием загрузки и её подтверждением.
func VideoOriginalObjectKey(videoID uuid.UUID) string {
	return "videos/" + videoID.String() + "/original"
}

// VideoHLSPrefix вычисляет префикс ключей объектов HLS-результатов обработки видео в
// хранилище (§3.3 эпика): мастер-манифест, медиаплейлисты и сегменты по профилям. Используется
// для best-effort зачистки результатов-сирот (§7.2) и повторной загрузки при обработке.
func VideoHLSPrefix(videoID uuid.UUID) string {
	return "videos/" + videoID.String() + "/hls/"
}

// VideoFailure описывает причину перевода видео в failed при таймауте watchdog'а (§8 дизайна
// эпика): класс ошибки и человекочитаемая причина.
type VideoFailure struct {
	Class  VideoFailureClass
	Reason string
}

// TimedOutReport — результат одного тика watchdog'а (§8 дизайна эпика): идентификаторы
// видео, переведённых в failed по каждому из трёх таймаутов.
type TimedOutReport struct {
	Uploading   []uuid.UUID
	Queued      []uuid.UUID
	Compressing []uuid.UUID
}

// VideoUpload — результат создания загрузки видео: идентификатор видео и преподписанный
// URL на PUT-загрузку оригинала.
type VideoUpload struct {
	VideoID   uuid.UUID
	UploadURL PreflightURL
	ExpiresAt time.Time
}

// VideoAccessKind определяет вид точки доступа к видео, возвращаемой Video.Get (§4.4 дизайна
// эпика): "hls" — через мастер-плейлист по HLS-токену, "original" — преподписанный GET оригинала.
type VideoAccessKind string

const (
	VideoAccessKindHLS      VideoAccessKind = "hls"
	VideoAccessKindOriginal VideoAccessKind = "original"
)

// VideoAccess — результат Video.Get: точка доступа к видео, выбранная по статусу видео и
// флагу is_prefer_original (§4.4 дизайна эпика). Handler собирает итоговый URL мастер-плейлиста
// из HLSToken (сервису неизвестен адрес API), для оригинала — использует URL как есть.
type VideoAccess struct {
	Kind VideoAccessKind
	// URL — преподписанный URL на оригинал в хранилище, заполняется только при Kind == original.
	URL PreflightURL
	// HLSToken — HLS-токен для запроса мастер-плейлиста, заполняется только при Kind == hls.
	HLSToken  string
	ExpiresAt time.Time
	Video     Video
	// Profiles — имена HLS-профилей видео по возрастанию качества, заполняется только при
	// Kind == hls.
	Profiles []string
}

// VideoPatch описывает опциональные поля частичного обновления видео при условном переходе
// статуса (см. Video.UpdateStatusIf репозитория). Незаполненные поля не изменяются.
type VideoPatch struct {
	// ProcessingAttempt — новое значение номера текущей попытки обработки.
	ProcessingAttempt *int
	// ExpectedAttempt — ожидаемое текущее значение processing_attempt в условии WHERE;
	// защищает от гонки между watchdog'ом и обработчиком событий.
	ExpectedAttempt *int
	FailureClass    *VideoFailureClass
	FailureReason   *string
	DurationMs      *int64
	Width           *int
	Height          *int
	// ClearFailure сбрасывает FailureClass и FailureReason в NULL.
	ClearFailure bool
	// QueuedAt — новое значение времени постановки в очередь (переход uploading → queued),
	// момент complete из метрики времени публикации (эпик Э5, Э5-Т5).
	QueuedAt *time.Time
	// CompressingStartedAt — новое значение времени взятия в обработку конвейером (переход
	// queued → compressing по событию ProcessingStarted, эпик Э5, исправление Д-1).
	CompressingStartedAt *time.Time
	// ReadyAt — новое значение времени готовности видео (переход compressing → ready по событию
	// ProcessingCompleted, эпик Э5, Э5-Т5) — финальная точка метрики времени публикации.
	ReadyAt *time.Time
}

type Video struct {
	ID                uuid.UUID
	GroupID           uuid.UUID
	Name              string
	Author            uuid.UUID
	Status            VideoStatus
	CreatedAt         time.Time
	StatusChangedAt   time.Time
	ProcessingAttempt int
	FailureClass      *VideoFailureClass
	FailureReason     *string
	DurationMs        *int64
	Width             *int
	Height            *int
	// IsUrgent — признак срочного видео: берётся в обработку приоритетной полосой мимо общей
	// очереди (эпик Э5, В-2).
	IsUrgent bool
	// QueuedAt — время постановки в очередь на обработку (переход uploading → queued), момент
	// complete из метрики времени публикации (эпик Э5, Э5-Т5).
	QueuedAt *time.Time
	// CompressingStartedAt — время взятия в обработку конвейером (переход queued → compressing
	// по событию ProcessingStarted).
	CompressingStartedAt *time.Time
	// ReadyAt — время готовности видео (переход compressing → ready по событию
	// ProcessingCompleted).
	ReadyAt *time.Time
}

// PipelineProgress — индикатор живости конвейера обработки видео для одной полосы (архивной
// или срочной): момент последнего успешно обработанного ProcessingStarted в этой полосе.
// Используется watchdog'ом, чтобы отличить «конвейер обрабатывает, очередь длинная» от
// «конвейер стоит» (эпик Э5, исправление Д-1).
type PipelineProgress struct {
	IsUrgent       bool
	LastDequeuedAt time.Time
}

// FromDB заполняет индикатор прогресса конвейера данными строки pipeline_progress.
func (p *PipelineProgress) FromDB(db *schema.PipelineProgress) {
	p.IsUrgent = db.IsUrgent
	p.LastDequeuedAt = db.LastDequeuedAt
}

// VideoAuthor — краткие сведения об авторе видео для отображения в карточке (П-6 контракта
// Э2): имя и фамилия резолвятся батчем по author-id видео (Video.SelectByIDs), поэтому не
// всегда доступны — при отсутствующем пользователе остаются пустыми, заполнен только ID.
type VideoAuthor struct {
	ID      uuid.UUID
	Name    string
	Surname string
}

// QueuePosition — место видео в очереди на обработку и размер полосы, к которой оно
// принадлежит (эпик Э5, В-3): архивная и срочная полосы считаются независимо друг от друга
// (см. Video.IsUrgent), поэтому Position/Total осмысленны только в пределах своей полосы, а не
// по системе в целом. Position — 1-based (первый в очереди имеет Position == 1).
type QueuePosition struct {
	// Position — место видео в очереди внутри своей полосы, считается по возрастанию
	// status_changed_at (момент постановки в очередь, FIFO по входу в полосу).
	Position int
	// Total — общий размер полосы (архивной или срочной), к которой относится видео.
	Total int
}

// VideoListItem — элемент списка видео группы (Э1-Т20, §5 дизайна эпика): видео вместе со
// сведениями, вычисляемыми из его ассетов (Profiles, HasProcessed), и причиной сбоя, видимой
// только инициатору с правом ManageVideo (Э1-Т17).
type VideoListItem struct {
	Video

	// Author — автор видео объектом (П-6 контракта Э2), затеняет промотированное поле
	// Video.Author (id создателя) полными сведениями об авторе.
	Author VideoAuthor
	// Profiles — имена HLS-профилей видео по возрастанию качества.
	Profiles []string
	// HasProcessed — признак наличия обработанной версии (готового мастер-плейлиста HLS).
	HasProcessed bool
	// Failure — причина сбоя обработки, заполняется только для инициатора с правом ManageVideo
	// (аккаунтным или групповым); для остальных остаётся nil, даже если видео в статусе failed.
	Failure *VideoFailure
	// QueuePosition — место видео в очереди на обработку в пределах своей полосы (эпик Э5,
	// В-3), заполняется только для видео в статусе queued; для остальных статусов — nil.
	QueuePosition *QueuePosition
}

type VideoAsset struct {
	FileID      uuid.UUID
	VideoID     uuid.UUID
	Kind        VideoAssetKind
	Profile     VideoProfile
	Bucket      string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

// OutboxEvent — событие, ожидающее публикации в брокер сообщений (transactional outbox).
type OutboxEvent struct {
	ID        int64
	Topic     string
	Key       string
	Payload   []byte
	CreatedAt time.Time
}

func (v *Video) FromDB(db *schema.UserGroupVideo) {
	v.ID = db.ID
	v.GroupID = db.UserGroupID
	v.Name = db.Name
	v.Author = db.Author
	v.Status = VideoStatus(db.Status)
	v.CreatedAt = db.CreatedAt
	v.StatusChangedAt = db.StatusChangedAt
	v.ProcessingAttempt = int(db.ProcessingAttempt)
	v.FailureReason = db.FailureReason.Ptr()
	v.DurationMs = db.DurationMS.Ptr()
	v.IsUrgent = db.IsUrgent
	v.QueuedAt = db.QueuedAt.Ptr()
	v.CompressingStartedAt = db.CompressingStartedAt.Ptr()
	v.ReadyAt = db.ReadyAt.Ptr()

	if p := db.FailureClass.Ptr(); p != nil {
		failureClass := VideoFailureClass(*p)
		v.FailureClass = &failureClass
	}
	if p := db.Width.Ptr(); p != nil {
		width := int(*p)
		v.Width = &width
	}
	if p := db.Height.Ptr(); p != nil {
		height := int(*p)
		v.Height = &height
	}
}

// FromDB заполняет ассет данными строки video_assets. Поля файла (Bucket, ObjectKey,
// ContentType, SizeBytes) остаются пустыми — используй FromDBWithFile, если они нужны.
func (va *VideoAsset) FromDB(db *schema.VideoAsset) {
	va.FileID = db.FileID
	va.VideoID = db.VideoID
	va.Kind = VideoAssetKind(db.Kind)
	va.Profile = VideoProfile(db.Profile)
	va.CreatedAt = db.CreatedAt
}

// FromDBWithFile заполняет ассет вместе с данными связанного файла (join video_assets + files).
func (va *VideoAsset) FromDBWithFile(db *schema.VideoAsset, file *schema.File) {
	va.FromDB(db)

	if file == nil {
		return
	}

	va.Bucket = file.Bucket
	va.ObjectKey = file.ObjectKey
	va.ContentType = file.ContentType
	va.SizeBytes = file.SizeBytes
}

func (e *OutboxEvent) FromDB(db *schema.OutboxEvent) {
	e.ID = db.ID
	e.Topic = db.Topic
	e.Key = db.Key
	e.Payload = []byte(db.Payload.Val)
	e.CreatedAt = db.CreatedAt
}
