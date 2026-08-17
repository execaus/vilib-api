package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetUserGroup godoc
// @Summary Карточка группы пользователей
// @Description Возвращает данные группы пользователей аккаунта. Доступно любому участнику аккаунта.
// @Tags user_groups
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Success 200 {object} dto.GetUserGroupResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId} [get]
func (h *Handler) GetUserGroup(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	groupID, err := h.GetPathUUIDValue(c, pathKeyUserGroupID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var group domain.UserGroup
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		group, err = services.UserGroup.Get(ctx, claims.UserID, accountID, groupID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	var dtoGroup dto.UserGroup
	dtoGroup.FromDomain(group)

	sendOK(c, dto.GetUserGroupResponse{Group: dtoGroup})
}
