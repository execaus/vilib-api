package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PostVideoProgress godoc
// @Summary Heartbeat прогресса просмотра видео
// @Description Принимает отрезок непрерывного воспроизведения от плеера и обновляет прогресс
// @Description просмотра инициатора по видео — усекает перемотку, ограничивает покрытие
// @Description временем у стены, отбрасывает интервалы со скоростью выше 1.0 и идемпотентен
// @Description при повторе (session_id, seq), §3 дизайна эпика Э3.
// @Tags watch-progress
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Param request body dto.WatchHeartbeatRequest true "Тело запроса"
// @Success 200 {object} dto.WatchProgressResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId}/progress [post]
func (h *Handler) PostVideoProgress(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	groupID, err := h.GetPathUUIDValue(c, pathKeyUserGroupID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	videoID, err := h.GetPathUUIDValue(c, pathKeyVideoID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var req dto.WatchHeartbeatRequest
	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var resp dto.WatchProgressResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		state, hbErr := services.WatchProgress.Heartbeat(
			ctx,
			accountID,
			groupID,
			claims.UserID,
			videoID,
			req.ToDomain(),
		)
		if hbErr != nil {
			zap.L().Error(hbErr.Error())
			return hbErr
		}

		resp.FromDomain(state)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
