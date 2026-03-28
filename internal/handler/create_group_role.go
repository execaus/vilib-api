package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *Handler) CreateGroupRole(c *gin.Context) {
	var req dto.CreateGroupRoleRequest

	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		role domain.GroupRole
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		role, err = services.GroupRole.Create(ctx, accountID, req.Name, req.PermissionMask, req.IsDefault)
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
