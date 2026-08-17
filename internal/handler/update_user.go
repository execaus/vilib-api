package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateUser godoc
// @Summary Обновление пользователя
// @Description Частично обновляет ФИО и/или роль пользователя. Смена роли и правка чужого
// @Description профиля требуют права ManageUsers; правка своего ФИО без смены роли разрешена
// @Description без прав. Все поля пустые — 200 без изменений.
// @Tags users
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userId path string true "ID пользователя"
// @Param request body dto.UpdateUserRequest true "Тело запроса для обновления пользователя"
// @Success 200 {object} dto.UpdateUserResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/users/{userId} [put]
func (h *Handler) UpdateUser(c *gin.Context) {
	var req dto.UpdateUserRequest

	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	targetUserID, err := h.GetPathUUIDValue(c, pathKeyUserID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var (
		user domain.User
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		patch := domain.UserPatch{
			Name:    req.Name,
			Surname: req.Surname,
			RoleID:  req.RoleID,
		}
		user, err = services.User.Update(ctx, claims.UserID, accountID, targetUserID, patch)
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
	dtoUser.FromDomain(user)

	sendOK(c, dto.UpdateUserResponse{
		User: dtoUser,
	})
}
