package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ResetPassword godoc
// @Summary Сброс пароля по токену
// @Description Обновляет пароль строки пользователя, которой принадлежит токен из письма.
// @Description Несуществующий, использованный или просроченный токен — 400 validation.reset_token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Тело запроса для сброса пароля"
// @Success 200 {object} dto.ResetPasswordResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500
// @Router /api/v1/auth/password/reset [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		if err := services.Auth.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.ResetPasswordResponse{})
}
