package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DeleteAccountRole godoc
// @Summary Удаление роли аккаунта
// @Description Удаляет роль аккаунта. Нельзя удалить системную роль или роль, назначенную активным пользователям.
// @Tags account_roles
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param roleId path string true "ID роли"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/roles/{roleId} [delete]
func (h *Handler) DeleteAccountRole(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	roleID, err := h.GetPathUUIDValue(c, pathKeyRoleID)
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

		if err = services.AccountRole.Delete(ctx, claims.UserID, accountID, roleID); err != nil {
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
