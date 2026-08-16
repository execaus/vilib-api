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

type Video struct {
	ID         uuid.UUID `json:"id"`
	GroupID    uuid.UUID `json:"group_id"`
	Name       string    `json:"name"`
	Author     uuid.UUID `json:"author"`
	Status     uint      `json:"status"`
	StatusName string    `json:"status_name"`
	CreatedAt  time.Time `json:"created_at"`
}

func (v *Video) FromDomain(video domain.Video) {
	v.ID = video.ID
	v.GroupID = video.GroupID
	v.Name = video.Name
	v.Author = video.Author
	v.Status = uint(video.Status)
	v.StatusName = video.Status.String()
	v.CreatedAt = video.CreatedAt
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
