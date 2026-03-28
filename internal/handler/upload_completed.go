package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/service"
)

func (h *Handler) UploadCompleted() {
	// Создаёт asset видео с тегом original
	h.saga.Run(ctx, func(ctx context.Context, services *service.Service) error {
		services.VideoAsset.Create(ctx, videoID, domain.VideoAssetTagOriginal, bucketName, contentType, sizeBytes)
	})
}
