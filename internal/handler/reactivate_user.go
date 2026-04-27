package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ReactivateUser godoc
// @Summary Реактивация пользователя
// @Description Реактивирует ранее деактивированного пользователя в аккаунте
// @Tags users
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userId path string true "ID пользователя"
// @Success 200 {object} dto.UpdateUserResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/users/{userId}/reactivate [post]
func (h *Handler) ReactivateUser(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	targetUserID, err := h.GetPathUUIDValue(c, pathKeyUserID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var user domain.User
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.User.Reactivate(ctx, claims.UserID, accountID, targetUserID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		// Получить обновлённого пользователя
		users, err := services.User.GetByID(ctx, targetUserID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}
		if len(users) > 0 {
			user = users[0]
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoUser := dto.User{}
	dtoUser.FromDomain(user)

	sendOK(c, dto.UpdateUserResponse{User: dtoUser})
}
