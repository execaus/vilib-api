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
// @Summary Создание загрузки видео
// @Description Создаёт запись видео в статусе загрузки и возвращает преподписанный URL для загрузки оригинала в S3
// @Tags videos
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Param request body dto.UploadVideoRequest true "Тело запроса"
// @Success 201 {object} dto.UploadVideoResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
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

	var request dto.UploadVideoRequest
	if err = c.BindJSON(&request); err != nil {
		sendBadRequest(c, err)
		return
	}

	var upload domain.VideoUpload
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		upload, err = services.Video.CreateUpload(
			ctx,
			accountID, groupID, claims.UserID,
			request.Name, request.ContentType, request.SizeBytes,
		)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendCreated(c, dto.UploadVideoResponse{
		VideoID:   upload.VideoID,
		UploadURL: upload.UploadURL,
		ExpiresAt: upload.ExpiresAt,
	})
}
