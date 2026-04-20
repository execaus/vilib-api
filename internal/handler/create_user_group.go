package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreateUserGroup godoc
// @Summary Создание группы пользователей
// @Description Создаёт новую группу пользователей и добавляет указанных пользователей в неё
// @Tags user-groups
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param request body dto.CreateUserGroupRequest true "Тело запроса для создания группы"
// @Success 201 {object} dto.CreateUserGroupResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/user-groups [post]
func (h *Handler) CreateUserGroup(c *gin.Context) {
	var req dto.CreateUserGroupRequest

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
		group   domain.UserGroup
		members []domain.GroupMember
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		group, err = services.UserGroup.Create(ctx, accountID, claims.UserID, req.Name)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		members, err = services.UserGroup.AddMembers(ctx, accountID, claims.UserID, group.ID, req.Users...)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	resp := dto.CreateUserGroupResponse{
		ID:   group.ID,
		Name: group.Name,
	}
	if len(members) != 0 {
		resp.Users = make([]dto.GroupMember, len(members))
		for i, user := range members {
			resp.Users[i] = dto.GroupMember{}
			resp.Users[i].FromDomain(user)
		}
	}

	sendCreated(c, resp)
}
