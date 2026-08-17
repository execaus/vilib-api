package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateUserGroup godoc
// @Summary Переименование группы пользователей
// @Description Переименовывает группу пользователей аккаунта. Право — ManageGroups.
// @Tags user_groups
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param request body dto.UpdateUserGroupRequest true "Тело запроса для переименования группы"
// @Success 200 {object} dto.UpdateUserGroupResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId} [put]
func (h *Handler) UpdateUserGroup(c *gin.Context) {
	var req dto.UpdateUserGroupRequest

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

	if err = c.BindJSON(&req); err != nil {
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

		group, err = services.UserGroup.Rename(ctx, claims.UserID, accountID, groupID, req.Name)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoGroup := dto.UserGroup{}
	dtoGroup.FromDomain(group)

	sendOK(c, dto.UpdateUserGroupResponse{Group: dtoGroup})
}
