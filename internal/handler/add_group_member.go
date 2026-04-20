package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AddGroupMember godoc
// @Summary Добавление участников в группу
// @Description Добавляет указанных пользователей в существующую группу
// @Tags user-groups
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Param request body dto.AddGroupMemberRequest true "Тело запроса для добавления участников"
// @Success 201 {object} dto.AddGroupMemberResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/members [post]
func (h *Handler) AddGroupMember(c *gin.Context) {
	var req dto.AddGroupMemberRequest

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

	var (
		members []domain.GroupMember
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		members, err = services.UserGroup.AddMembers(ctx, accountID, claims.UserID, groupID, req.Users...)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoMembers := make([]dto.GroupMember, len(members))
	for i, member := range members {
		dtoMembers[i] = dto.GroupMember{}
		dtoMembers[i].FromDomain(member)
	}

	sendCreated(c, dto.AddGroupMemberResponse{
		Members: dtoMembers,
	})
}
