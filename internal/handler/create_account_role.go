package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *Handler) CreateAccountRole(c *gin.Context) {
	var req dto.CreateAccountRoleRequest

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
		accountRoles []domain.AccountRole
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		accountRoles, err = services.AccountRole.Create(
			ctx,
			accountID, req.Name, req.ParentID, req.Permission, req.IsDefault, false,
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

	dtoAccountRoles := make([]dto.AccountRole, len(accountRoles))
	for i, role := range accountRoles {
		dtoAccountRoles[i] = dto.AccountRole{}
		dtoAccountRoles[i].FromDomain(role)
	}

	sendCreated(c, dto.CreateAccountRoleResponse{
		AccountRoles: dtoAccountRoles,
	})
}
