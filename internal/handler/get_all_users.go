package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetAllUsers godoc
// @Summary Список пользователей аккаунта
// @Description Возвращает список пользователей, привязанных к аккаунту, с опциональной фильтрацией по статусу
// @Tags users
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param status query string false "Фильтр по статусу: active, deactivated, all (по умолчанию: active)"
// @Success 200 {object} dto.GetAllUsersResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Router /api/v1/accounts/{accountId}/users [get]
func (h *Handler) GetAllUsers(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var req dto.GetAllUsersRequest
	if err = c.ShouldBindQuery(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	// Дефолтный статус — active
	if req.Status == "" {
		req.Status = string(repository.UserStatusActive)
	}

	var users []dto.User
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		domainUsers, err := services.User.ListByAccount(ctx, claims.UserID, accountID, repository.UserStatus(req.Status))
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		users = make([]dto.User, len(domainUsers))
		for i, u := range domainUsers {
			users[i] = dto.User{}
			users[i].FromDomain(u)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.GetAllUsersResponse{Users: users})
}
