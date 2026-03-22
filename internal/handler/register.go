package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Register godoc
// @Summary Регистрация нового пользователя
// @Description Создаёт пользователя, аккаунт, назначает роль администратора и возвращает токен авторизации
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Тело запроса для регистрации"
// @Success 201 {object} dto.RegisterResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		if _, err := services.Account.Create(ctx, req.Name, req.Surname, req.Email); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendCreated(c, dto.RegisterResponse{})
}
