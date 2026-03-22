package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateUser godoc
// @Summary Обновление пользователя
// @Description Обновляет данные пользователя (например, статус аккаунта)
// @Tags users
// @Accept json
// @Produce json
// @Param user_id path string true "ID пользователя"
// @Param request body dto.UpdateUserRequest true "Тело запроса для обновления пользователя"
// @Success 200 {object} dto.UpdateUserResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/users/{user_id} [put]
func (h *Handler) UpdateUser(c *gin.Context) {
	var req dto.UpdateUserRequest

	targetUserID, err := h.GetPathStringValue(c, pathKeyUserID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		user          domain.User
		accountStatus domain.AccountStatus
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		user, err = services.User.Update(ctx, claims.UserID, targetUserID, req.StatusPosition)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accountStatus, err = sliceItemsToSingle(func() ([]domain.AccountStatus, error) {
			return services.AccountStatus.GetByUsersID(ctx, user.ID)
		})

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoUser := dto.User{}
	dtoUser.FromDomain(user, accountStatus.Status)

	sendOK(c, dto.UpdateUserResponse{
		User: dtoUser,
	})
}
