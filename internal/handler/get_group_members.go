package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetGroupMembers godoc
// @Summary Список участников группы
// @Description Возвращает участников группы вместе с их ролью и правами. Требуется право ManageGroups аккаунта или ManageMembers группы.
// @Tags group_members
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Success 200 {object} dto.GetGroupMembersResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/members [get]
func (h *Handler) GetGroupMembers(c *gin.Context) {
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

	var members []domain.GroupMemberDetails
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		members, err = services.GroupMember.ListByGroup(ctx, accountID, claims.UserID, groupID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoMembers := make([]dto.GroupMemberDetails, len(members))
	for i, member := range members {
		dtoMembers[i].FromDomain(member)
	}

	sendOK(c, dto.GetGroupMembersResponse{Members: dtoMembers})
}
