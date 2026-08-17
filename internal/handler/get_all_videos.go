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
// @Description Возвращает список видео в группе пользователей: статус, профили HLS и признак
// @Description наличия обработанной версии — из ассетов видео. Причина сбоя (failure) видна
// @Description только инициатору с правом ManageVideo, иначе поле — null (Э1-Т17).
// @Tags videos
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Success 200 {object} dto.GetAllVideosResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
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
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		items, videosErr := services.Video.GetAll(ctx, accountID, groupID, claims.UserID)
		if videosErr != nil {
			zap.L().Error(videosErr.Error())
			return videosErr
		}

		videos = make([]dto.Video, len(items))
		for i, item := range items {
			videos[i] = dto.Video{}
			videos[i].FromDomainListItem(item)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.GetAllVideosResponse{Videos: videos})
}
