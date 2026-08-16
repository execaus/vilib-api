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
	DefaultVideoName  = "untitled"
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
