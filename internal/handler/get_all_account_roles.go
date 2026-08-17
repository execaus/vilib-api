package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetAllAccountRoles godoc
// @Summary Список ролей аккаунта
// @Description Возвращает список всех ролей аккаунта
// @Tags account_roles
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Success 200 {array} dto.AccountRole
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/roles [get]
func (h *Handler) GetAllAccountRoles(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var roles []dto.AccountRole
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		domainRoles, rolesErr := services.AccountRole.GetAll(ctx, claims.UserID, accountID)
		if rolesErr != nil {
			zap.L().Error(rolesErr.Error())
			return rolesErr
		}

		roles = make([]dto.AccountRole, len(domainRoles))
		for i, r := range domainRoles {
			roles[i] = dto.AccountRole{}
			roles[i].FromDomain(r)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, roles)
}
