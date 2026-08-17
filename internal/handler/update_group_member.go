package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateGroupMember godoc
// @Summary Смена роли участника группы
// @Description Меняет роль участника в группе. Требуется право ManageGroups аккаунта или ManageMembers группы.
// @Tags group_members
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param memberId path string true "ID пользователя-участника"
// @Param request body dto.UpdateGroupMemberRequest true "Тело запроса смены роли участника"
// @Success 200 {object} dto.UpdateGroupMemberResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/members/{memberId} [put]
func (h *Handler) UpdateGroupMember(c *gin.Context) {
	var req dto.UpdateGroupMemberRequest

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

	targetUserID, err := h.GetPathUUIDValue(c, pathKeyGroupMemberUserID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var member domain.GroupMemberDetails
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		member, err = services.GroupMember.UpdateRole(ctx, accountID, claims.UserID, groupID, targetUserID, req.RoleID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	var dtoMember dto.GroupMemberDetails
	dtoMember.FromDomain(member)

	sendOK(c, dto.UpdateGroupMemberResponse{Member: dtoMember})
}
