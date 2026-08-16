package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetVideoHLSPlaylist godoc
// @Summary Медиаплейлист HLS-профиля
// @Description Отдаёт медиаплейлист HLS-профиля видео, переписывая имена сегментов
// @Description преподписанными URL хранилища. Доступ проверяется HLS-токеном в query — ручка
// @Description не проходит RequireAuthMiddleware (§4.2 дизайна эпика).
// @Tags videos
// @Produce application/vnd.apple.mpegurl
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Param videoId path string true "ID видео"
// @Param profile path string true "Имя профиля качества (например, 720p)"
// @Param token query string true "HLS-токен, выданный GET-ручкой видео"
// @Success 200 {string} string "Медиаплейлист HLS-профиля"
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/video/{videoId}/hls/{profile}/playlist.m3u8 [get]
func (h *Handler) GetVideoHLSPlaylist(c *gin.Context) {
	videoID, err := h.GetPathUUIDValue(c, pathKeyVideoID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	profile, err := h.GetPathStringValue(c, pathKeyProfile)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	token, err := hlsTokenFromQuery(c)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var body []byte
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		body, err = services.Video.GetHLSPlaylist(ctx, videoID, domain.VideoProfile(profile), token)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendHLSPlaylist(c, body)
}
