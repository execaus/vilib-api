package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreateGroupRole godoc
// @Summary Создание роли группы
// @Description Создаёт новую роль для группы пользователей с указанными правами
// @Tags group_roles
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param request body dto.CreateGroupRoleRequest true "Тело запроса для создания роли группы"
// @Success 201 {object} dto.CreateGroupRoleResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/user-groups/roles [post]
func (h *Handler) CreateGroupRole(c *gin.Context) {
	var req dto.CreateGroupRoleRequest

	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		role domain.GroupRole
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		role, err = services.GroupRole.Create(
			ctx,
			accountID,
			claims.UserID,
			req.Name,
			req.PermissionMask,
			req.IsDefault,
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

	sendCreated(c, dto.CreateGroupRoleResponse{
		Role: dtoRole,
	})
}
