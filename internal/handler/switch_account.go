package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SwitchAccount godoc
// @Summary Переключение организации
// @Description Выпускает новый токен с текущей организацией сессии, переключённой на
// @Description организацию, где у пользователя есть активная строка (§2.4 дизайна эпика Э2)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.SwitchAccountRequest true "Организация для переключения"
// @Success 200 {object} dto.SwitchAccountResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/auth/switch-account [post]
func (h *Handler) SwitchAccount(c *gin.Context) {
	var req dto.SwitchAccountRequest

	if err := c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var token string
	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		var switchErr error
		token, switchErr = services.Auth.SwitchAccount(ctx, claims.UserID, req.AccountID)
		if switchErr != nil {
			zap.L().Error(switchErr.Error())
			return switchErr
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.SwitchAccountResponse{Token: token})
}
