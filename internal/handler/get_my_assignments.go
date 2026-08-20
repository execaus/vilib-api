package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetMyAssignments godoc
// @Summary Мои назначения
// @Description Возвращает назначения текущего пользователя во всех статусах вместе со
// @Description сводкой активных и просроченных (§4, §5 дизайна эпика Э3) — используется для
// @Description экрана «Мои назначения», бейджей в каталоге/плеере и счётчика в навигации.
// @Tags assignments
// @Produce json
// @Success 200 {object} dto.MyAssignmentsResponse
// @Failure 401
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/me/assignments [get]
func (h *Handler) GetMyAssignments(c *gin.Context) {
	var resp dto.MyAssignmentsResponse

	if err := h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		items, listErr := services.Assignment.ListMine(ctx, claims.UserID)
		if listErr != nil {
			zap.L().Error(listErr.Error())
			return listErr
		}

		resp.FromDomain(items)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
