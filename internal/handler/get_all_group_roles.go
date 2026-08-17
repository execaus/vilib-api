package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetAllGroupRoles godoc
// @Summary Список ролей групп
// @Description Возвращает список всех ролей групп для аккаунта
// @Tags group_roles
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Success 200 {array} dto.GroupRole
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/roles [get]
func (h *Handler) GetAllGroupRoles(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var roles []dto.GroupRole
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		domainRoles, rolesErr := services.GroupRole.GetAll(ctx, claims.UserID, accountID)
		if rolesErr != nil {
			zap.L().Error(rolesErr.Error())
			return rolesErr
		}

		roles = make([]dto.GroupRole, len(domainRoles))
		for i, r := range domainRoles {
			roles[i] = dto.GroupRole{}
			roles[i].FromDomain(r)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, roles)
}
