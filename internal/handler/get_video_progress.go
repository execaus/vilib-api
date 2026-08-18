package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetVideoProgress godoc
// @Summary Текущий прогресс просмотра видео
// @Description Возвращает текущее состояние прогресса просмотра инициатора по видео без
// @Description изменений — нет накопленного прогресса означает нулевое состояние (§3 дизайна
// @Description эпика Э3).
// @Tags watch-progress
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Success 200 {object} dto.WatchProgressResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId}/progress [get]
func (h *Handler) GetVideoProgress(c *gin.Context) {
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

	var resp dto.WatchProgressResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		state, getErr := services.WatchProgress.Get(ctx, accountID, groupID, claims.UserID, videoID)
		if getErr != nil {
			zap.L().Error(getErr.Error())
			return getErr
		}

		resp.FromDomain(state)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
