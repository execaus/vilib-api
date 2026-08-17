package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetAllUserGroups godoc
// @Summary Список групп пользователей
// @Description Возвращает список групп пользователей аккаунта
// @Tags user_groups
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Success 200 {object} dto.GetAllUserGroupsResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups [get]
func (h *Handler) GetAllUserGroups(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var groups []dto.UserGroup
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		domainGroups, groupsErr := services.UserGroup.GetAll(ctx, claims.UserID, accountID)
		if groupsErr != nil {
			zap.L().Error(groupsErr.Error())
			return groupsErr
		}

		groups = make([]dto.UserGroup, len(domainGroups))
		for i, g := range domainGroups {
			groups[i].FromDomain(g)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.GetAllUserGroupsResponse{Groups: groups})
}
