package handler

import (
	"context"
	"errors"
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
		token string
	)
	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		user, err := services.User.GetByEmail(ctx, req.Email)
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				zap.L().Warn(ErrInvalidCredentials.Error())
				return ErrInvalidCredentials
			}
			zap.L().Error(err.Error())
			return err
		}

		if ok := services.Auth.ComparePassword(ctx, user.PasswordHash, req.Password); !ok {
			zap.L().Warn(ErrInvalidCredentials.Error())
			return ErrInvalidCredentials
		}

		accounts, err := services.Account.GetByUserEmail(ctx, user.Email)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if len(accounts) == 0 {
			zap.L().Error("accounts not found")
			return errors.New("accounts not found")
		}

		accountsID := make([]string, len(accounts))
		for i := 0; i < len(accounts); i++ {
			accountsID[i] = accounts[i].ID
		}

		token, err = services.Auth.GenerateToken(ctx, accountsID, user.ID, accountsID[0])
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.LoginResponse{Token: token})
}
