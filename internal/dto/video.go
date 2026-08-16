package dto

import (
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

// UploadVideoRequest — тело запроса на создание загрузки видео.
type UploadVideoRequest struct {
	Name        string `json:"name"         binding:"required,min=1,max=255"`
	ContentType string `json:"content_type" binding:"required,startswith=video/"`
	SizeBytes   int64  `json:"size_bytes"   binding:"required,gt=0"`
}

// UploadVideoResponse — ответ на создание загрузки видео: идентификатор видео и
// преподписанный URL на PUT-загрузку оригинала в хранилище.
type UploadVideoResponse struct {
	VideoID   uuid.UUID           `json:"video_id"`
	UploadURL domain.PreflightURL `json:"upload_url"`
	ExpiresAt time.Time           `json:"expires_at"`
}

type GetVideoQuery struct {
	IsPreferOriginal bool `form:"is_prefer_original"`
}

// GetVideoResponse — ответ на получение точки доступа к видео (§4.4, §5 дизайна эпика).
// Kind — "hls" (URL ведёт на мастер-плейлист с HLS-токеном в query) или "original"
// (преподписанный GET оригинала). Profiles заполняется только для Kind == "hls".
type GetVideoResponse struct {
	Kind       string    `json:"kind"`
	URL        string    `json:"url"`
	ExpiresAt  time.Time `json:"expires_at"`
	Status     uint      `json:"status"`
	StatusName string    `json:"status_name"`
	Profiles   []string  `json:"profiles"`
}

// CompleteVideoUploadResponse — ответ на подтверждение загрузки видео.
type CompleteVideoUploadResponse struct {
	Video Video `json:"video"`
}

// VideoFailure — причина сбоя обработки видео (Э1-Т17): класс ошибки и человекочитаемый текст.
// Присутствует в ответе только у инициатора с правом ManageVideo, иначе поле — null.
type VideoFailure struct {
	// Class — класс ошибки: permanent, temporary, timeout.
	Class string `json:"class"`
	// Reason — человекочитаемая причина сбоя.
	Reason string `json:"reason"`
}

type Video struct {
	ID         uuid.UUID `json:"id"`
	GroupID    uuid.UUID `json:"group_id"`
	Name       string    `json:"name"`
	Author     uuid.UUID `json:"author"`
	Status     uint      `json:"status"`
	StatusName string    `json:"status_name"`
	CreatedAt  time.Time `json:"created_at"`
	// Profiles — имена HLS-профилей видео по возрастанию качества (пустой список, пока
	// профилей нет).
	Profiles []string `json:"profiles"`
	// HasProcessed — признак наличия обработанной версии (готового мастер-плейлиста HLS).
	HasProcessed bool `json:"has_processed"`
	// DurationMs — длительность видео в миллисекундах, известна после успешной обработки.
	DurationMs *int64 `json:"duration_ms,omitempty"`
	// Width — ширина кадра оригинала в пикселях, известна после успешной обработки.
	Width *int `json:"width,omitempty"`
	// Height — высота кадра оригинала в пикселях, известна после успешной обработки.
	Height *int `json:"height,omitempty"`
	// Failure — причина сбоя обработки видео, заполняется только для инициатора с правом
	// ManageVideo (Э1-Т17); для остальных — всегда null, даже если видео в статусе failed.
	Failure *VideoFailure `json:"failure,omitempty"`
}

// FromDomain заполняет DTO базовыми полями видео без сведений об ассетах (Profiles — пустой
// список, HasProcessed — false, Failure — nil). Используется там, где ассеты видео не
// загружались (например, CompleteVideoUpload, RenameVideo) — для полного представления с
// профилями и причиной сбоя используй FromDomainListItem.
func (v *Video) FromDomain(video domain.Video) {
	v.ID = video.ID
	v.GroupID = video.GroupID
	v.Name = video.Name
	v.Author = video.Author
	v.Status = uint(video.Status)
	v.StatusName = video.Status.String()
	v.CreatedAt = video.CreatedAt
	v.Profiles = []string{}
}

// FromDomainListItem заполняет DTO элементом списка видео (Э1-Т20): профили, признак обработки
// и причина сбоя (если она была вычислена сервисом для инициатора с правом ManageVideo, §5
// дизайна эпика, Э1-Т17).
func (v *Video) FromDomainListItem(item domain.VideoListItem) {
	v.FromDomain(item.Video)

	v.Profiles = item.Profiles
	if v.Profiles == nil {
		v.Profiles = []string{}
	}
	v.HasProcessed = item.HasProcessed
	v.DurationMs = item.DurationMs
	v.Width = item.Width
	v.Height = item.Height

	if item.Failure != nil {
		v.Failure = &VideoFailure{Class: string(item.Failure.Class), Reason: item.Failure.Reason}
	}
}

type GetAllVideosResponse struct {
	Videos []Video `json:"videos"`
}

type RenameVideoRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

type RenameVideoResponse struct {
	Video Video `json:"video"`
}
