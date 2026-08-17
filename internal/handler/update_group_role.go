package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateGroupRole godoc
// @Summary Редактирование роли группы
// @Description Полностью заменяет роль группы. В отличие от роли аккаунта бит владельца
// @Description группы в маске прав разрешён. У аккаунта всегда должна остаться ровно одна
// @Description роль группы по умолчанию.
// @Tags group_roles
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupRoleId path string true "ID роли группы"
// @Param request body dto.UpdateGroupRoleRequest true "Тело запроса для редактирования роли группы"
// @Success 200 {object} dto.UpdateGroupRoleResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/roles/{groupRoleId} [put]
func (h *Handler) UpdateGroupRole(c *gin.Context) {
	var req dto.UpdateGroupRoleRequest

	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	roleID, err := h.GetPathUUIDValue(c, pathKeyGroupRoleID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var role domain.GroupRole
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		role, err = services.GroupRole.Update(
			ctx,
			claims.UserID, accountID, roleID,
			req.Name, req.PermissionMask, req.IsDefault,
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

	dtoRole := dto.GroupRole{}
	dtoRole.FromDomain(role)

	sendOK(c, dto.UpdateGroupRoleResponse{
		Role: dtoRole,
	})
}
