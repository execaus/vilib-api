package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

	accountID, err := h.GetPathStringValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		user          domain.User
		accountStatus domain.AccountStatus
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		user, err = services.Account.CreateUser(ctx, accountID, req.Name, req.Surname, req.Email)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		accountStatus, err = sliceItemsToSingle(func() ([]domain.AccountStatus, error) {
			return services.AccountStatus.GetByUsersID(ctx, user.ID)
		})
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	dtoUser := dto.User{}
	dtoUser.FromDomain(user, accountStatus.Status)

	sendCreated(c, dto.CreateUserResponse{
		User: dtoUser,
	})
}
