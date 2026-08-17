package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CompleteVideoUpload godoc
// @Summary Подтверждение загрузки видео
// @Description Подтверждает, что оригинал видео загружен в хранилище: проверяет объект,
// @Description регистрирует ассет-оригинал и переводит видео в очередь на обработку.
// @Description Повторный вызов для видео, уже поставленного в очередь, обрабатываемого или готового, идемпотентен.
// @Tags videos
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Param videoId path string true "ID видео"
// @Success 200 {object} dto.CompleteVideoUploadResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/video/{videoId}/complete [post]
func (h *Handler) CompleteVideoUpload(c *gin.Context) {
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

	var item domain.VideoListItem
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		item, err = services.Video.CompleteUpload(ctx, accountID, groupID, claims.UserID, videoID)
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
	dtoVideo.FromDomainListItem(item)

	sendOK(c, dto.CompleteVideoUploadResponse{Video: dtoVideo})
}
