package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/service"
)

func (h *Handler) CompressionCompleted() {
	h.saga.Run(ctx, func(ctx context.Context, services *service.Service) error {
		services.VideoAsset.Create(ctx, videoID, domain.VideoAssetTagCompressed, bucketName, contentType, bytes)

		status := domain.VideoStatusReady
		services.Video.Update(ctx, videoID, &status)
	})
}
