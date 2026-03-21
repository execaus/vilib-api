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

	var (
		token string
	)
	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		password, err := services.Auth.GeneratePassword()
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		passwordHash, err := services.Auth.HashPassword(ctx, password)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		user, err := services.User.Create(ctx, req.Name, req.Surname, req.Email, passwordHash)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		account, err := services.Account.Create(ctx, user.Email)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.User.IssueAdmin(ctx, user.ID, account.ID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accounts, err := services.Account.GetByUserEmail(ctx, user.Email)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accountsID := make([]string, len(accounts))
		for i := 0; i < len(accounts); i++ {
			accountsID[i] = accounts[i].ID
		}

		token, err = services.Auth.GenerateToken(ctx, accountsID, user.ID, account.ID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.Email.SendRegisteredMail(ctx, user.Email, password); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendCreated(c, dto.RegisterResponse{Token: token})
}
