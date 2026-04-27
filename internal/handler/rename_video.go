package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RenameVideo godoc
// @Summary Переименование видео
// @Description Переименовывает видео в группе
// @Tags videos
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Param request body dto.RenameVideoRequest true "Тело запроса"
// @Success 200 {object} dto.RenameVideoResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId} [put]
func (h *Handler) RenameVideo(c *gin.Context) {
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

	var req dto.RenameVideoRequest
	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var video domain.Video
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		video, err = services.Video.Rename(ctx, accountID, groupID, claims.UserID, videoID, req.Name)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoVideo := dto.Video{}
	dtoVideo.FromDomain(video)

	sendOK(c, dto.RenameVideoResponse{Video: dtoVideo})
}
