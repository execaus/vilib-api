package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetMe godoc
// @Summary Профиль текущего пользователя
// @Description Возвращает агрегированный контекст текущего пользователя: организацию, роль,
// @Description признак владельца и членства в группах (§2.3 дизайна эпика Э2)
// @Tags profile
// @Produce json
// @Success 200 {object} dto.MeResponse
// @Failure 401 {object} dto.ErrorMessage
// @Failure 403 {object} dto.ErrorMessage
// @Failure 500 {object} dto.ErrorMessage
// @Security BearerAuth
// @Router /api/v1/me [get]
func (h *Handler) GetMe(c *gin.Context) {
	var response dto.MeResponse

	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		profile, profileErr := services.Profile.Get(ctx, claims.UserID)
		if profileErr != nil {
			zap.L().Error(profileErr.Error())
			return profileErr
		}

		response.FromDomain(profile)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, response)
}
