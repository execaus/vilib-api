package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetVideoHLSMaster godoc
// @Summary Мастер-плейлист HLS
// @Description Отдаёт мастер-плейлист HLS-выдачи видео, переписывая URI вариантов ссылками
// @Description с тем же HLS-токеном. Доступ проверяется токеном в query, а не заголовком
// @Description Authorization — ручка не проходит RequireAuthMiddleware (§4.2 дизайна эпика).
// @Tags videos
// @Produce application/vnd.apple.mpegurl
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Param videoId path string true "ID видео"
// @Param token query string true "HLS-токен, выданный GET-ручкой видео"
// @Success 200 {string} string "Мастер-плейлист HLS"
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/video/{videoId}/hls/master.m3u8 [get]
func (h *Handler) GetVideoHLSMaster(c *gin.Context) {
	videoID, err := h.GetPathUUIDValue(c, pathKeyVideoID)
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
		body, err = services.Video.GetHLSMaster(ctx, videoID, token)
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
