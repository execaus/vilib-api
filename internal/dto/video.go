package dto

import (
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type UploadVideoResponse struct {
	PresignedURL domain.PreflightURL `json:"presigned_url"`
}

type GetVideoQuery struct {
	IsPreferOriginal bool `form:"is_prefer_original"`
}

type GetVideoResponse struct {
	PresignedURL domain.PreflightURL `json:"presigned_url"`
}

type Video struct {
	ID        uuid.UUID `json:"id"`
	GroupID   uuid.UUID `json:"group_id"`
	Name      string    `json:"name"`
	Author    uuid.UUID `json:"author"`
	Status    uint      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (v *Video) FromDomain(video domain.Video) {
	v.ID = video.ID
	v.GroupID = video.GroupID
	v.Name = video.Name
	v.Author = video.Author
	v.Status = uint(video.Status)
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
