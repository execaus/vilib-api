package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DeactivateUser godoc
// @Summary Деактивация пользователя
// @Description Деактивирует пользователя в аккаунте (мягкое удаление)
// @Tags users
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userId path string true "ID пользователя"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/users/{userId} [delete]
func (h *Handler) DeactivateUser(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	targetUserID, err := h.GetPathUUIDValue(c, pathKeyUserID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.User.Deactivate(ctx, claims.UserID, accountID, targetUserID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendNoContent(c)
}
