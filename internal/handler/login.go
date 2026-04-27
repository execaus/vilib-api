package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Login godoc
// @Summary Вход пользователя
// @Description Аутентифицирует пользователя по email и паролю и возвращает токен авторизации
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Тело запроса для входа"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		err   error
		token string
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		token, err = services.Auth.Login(ctx, req.Email, req.Password)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceErrorWithDeactivated(c, err)
		return
	}

	sendOK(c, dto.LoginResponse{Token: token})
}
