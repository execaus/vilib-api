package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UploadVideo godoc
// @Summary Загрузка видео
// @Description Возвращает преподписанный URL для загрузки видео в S3
// @Tags video
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Success 200 {object} dto.UploadVideoResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/video [post]
func (h *Handler) UploadVideo(c *gin.Context) {
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

	var (
		preflightURL domain.PreflightURL
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		preflightURL, err = services.Video.GetPreflightUploadURL(ctx, accountID, groupID, claims.UserID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.UploadVideoResponse{
		PresignedURL: preflightURL,
	})
}
