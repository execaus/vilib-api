package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

// CreateUser godoc
// @Summary Создание пользователя
// @Description Создает нового пользователя с указанными данными, привязывает его к аккаунту и отправляет email с паролем
// @Tags users
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта, к которому привязывается пользователь"
// @Param request body dto.CreateUserRequest true "Тело запроса для создания пользователя"
// @Success 201 {object} dto.CreateUserResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/users [post]
func (h *Handler) CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest

	reqAccountID, err := h.GetPathStringValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		user domain.User
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		password, err := services.Auth.GeneratePassword()
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accounts, err := services.Account.GetByUserEmail(ctx, req.Email)
		if slices.IndexFunc(accounts, func(account domain.Account) bool {
			return account.ID == reqAccountID
		}) != -1 {
			// TODO перенести логику в сервисы
			e := service.NewServiceError("user exists in the account")
			zap.L().Error(e.Error())
			return e
		}

		user, err = services.User.Create(ctx, req.Name, req.Surname, req.Email, password)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.User.IssueUser(ctx, user.ID, reqAccountID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		if err = services.Email.SendCreateUserEmail(ctx, req.Email, password); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoUser := dto.User{}
	dtoUser.FromDomain(user)

	sendCreated(c, dto.CreateUserResponse{
		User: dtoUser,
	})
}
