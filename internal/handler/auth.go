package handler

import (
	"context"
	"errors"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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

		account, err := services.Account.Create(ctx, user.ID, user.Email)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.User.IssueAdmin(ctx, user.ID, account.ID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accounts, err := services.Account.GetByUserID(ctx, user.ID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accountIDs := make([]string, len(accounts))
		for i := 0; i < len(accounts); i++ {
			accountIDs[i] = accounts[i].ID
		}

		token, err = services.Auth.GenerateToken(ctx, accountIDs, user.ID, account.ID)
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

		accounts, err := services.Account.GetByUserID(ctx, user.ID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if len(accounts) == 0 {
			zap.L().Error("accounts not found")
			return errors.New("accounts not found")
		}

		accountIDs := make([]string, len(accounts))
		for i := 0; i < len(accounts); i++ {
			accountIDs[i] = accounts[i].ID
		}

		token, err = services.Auth.GenerateToken(ctx, accountIDs, user.ID, accountIDs[0])
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
