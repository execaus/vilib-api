package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetAllVideos godoc
// @Summary Список видео группы
// @Description Возвращает список видео в группе пользователей
// @Tags videos
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Success 200 {object} dto.GetAllVideosResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/videos [get]
func (h *Handler) GetAllVideos(c *gin.Context) {
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

	var videos []dto.Video
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		domainVideos, err := services.Video.GetAll(ctx, accountID, groupID, claims.UserID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		videos = make([]dto.Video, len(domainVideos))
		for i, v := range domainVideos {
			videos[i] = dto.Video{}
			videos[i].FromDomain(v)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.GetAllVideosResponse{Videos: videos})
}
