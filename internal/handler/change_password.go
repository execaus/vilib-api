package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ChangePassword godoc
// @Summary Смена пароля
// @Description Меняет пароль текущей строки пользователя (организация из JWT). Неверный старый
// @Description пароль или новый пароль, совпадающий со старым либо короче минимальной длины —
// @Description 400.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Тело запроса для смены пароля"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/auth/password/change [post]
func (h *Handler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		if err := services.Auth.ChangePassword(ctx, claims.UserID, req.OldPassword, req.NewPassword); err != nil {
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
