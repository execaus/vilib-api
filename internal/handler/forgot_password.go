package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ForgotPassword godoc
// @Summary Запрос сброса пароля
// @Description Отправляет письмо со ссылкой (или списком ссылок по организациям, если email
// @Description состоит в нескольких) для сброса пароля. Ответ всегда 200, независимо от
// @Description существования email или организации.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Тело запроса для сброса пароля"
// @Success 200 {object} dto.ForgotPasswordResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500
// @Router /api/v1/auth/password/forgot [post]
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		if err := services.Auth.RequestPasswordReset(ctx, req.Email, req.AccountID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.ForgotPasswordResponse{})
}
