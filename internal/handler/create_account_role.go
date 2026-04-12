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
		accountRole domain.AccountRole
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accountRole, err = services.AccountRole.Create(
			ctx,
			accountID, claims.UserID,
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

	sendCreated(c, dto.CreateAccountRoleResponse{
		AccountRole: dtoAccountRole,
	})
}
