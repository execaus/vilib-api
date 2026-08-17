package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DeleteGroupMember godoc
// @Summary Удаление участника группы
// @Description Удаляет пользователя из группы
// @Tags group_members
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param memberId path string true "ID пользователя-участника"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/members/{memberId} [delete]
func (h *Handler) DeleteGroupMember(c *gin.Context) {
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

	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		if err = services.GroupMember.RemoveMember(ctx, accountID, claims.UserID, groupID, targetUserID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendNoContent(c)
}
