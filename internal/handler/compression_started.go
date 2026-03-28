package handler

func (h *Handler) CompressionStarted() {
	// Обновляет запись video.status = compressing
	//h.saga.Run(ctx, func(ctx context.Context, services *service.Service) error {
	//	status := domain.VideoStatusCompressing
	//	services.Video.Update(ctx, videoID, &status)
	//})
}
