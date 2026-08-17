package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateAccountRole godoc
// @Summary Редактирование роли аккаунта
// @Description Полностью заменяет роль аккаунта. Системную роль редактировать нельзя, бит
// @Description владельца в маске прав запрещён, у аккаунта всегда должна остаться ровно одна
// @Description роль по умолчанию.
// @Tags account_roles
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param roleId path string true "ID роли"
// @Param request body dto.UpdateAccountRoleRequest true "Тело запроса для редактирования роли"
// @Success 200 {object} dto.UpdateAccountRoleResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/roles/{roleId} [put]
func (h *Handler) UpdateAccountRole(c *gin.Context) {
	var req dto.UpdateAccountRoleRequest

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

	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var accountRole domain.AccountRole
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		accountRole, err = services.AccountRole.Update(
			ctx,
			claims.UserID, accountID, roleID,
			req.Name, req.ParentID, req.Permission, req.IsDefault,
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

	dtoAccountRole := dto.AccountRole{}
	dtoAccountRole.FromDomain(accountRole)

	sendOK(c, dto.UpdateAccountRoleResponse{
		AccountRole: dtoAccountRole,
	})
}
