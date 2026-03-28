package dto

import "vilib-api/internal/domain"

type UploadVideoResponse struct {
	PresignedURL domain.PreflightURL `json:"presigned_url"`
}

type GetVideoQuery struct {
	IsPreferOriginal bool `form:"is_prefer_original"`
}

type GetVideoResponse struct {
	PresignedURL domain.PreflightURL `json:"presigned_url"`
}
