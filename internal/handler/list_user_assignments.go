package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ListUserAssignments godoc
// @Summary Отчёт по назначениям сотрудника
// @Description Возвращает все назначения одного сотрудника с учётом области В-8 (§4, §6
// @Description дизайна эпика Э3, В-53): назначивший видит свои назначения этого сотрудника
// @Description всегда, иначе — только назначения из своей области (аккаунт целиком или список
// @Description групп). Сотрудник не состоит в аккаунте — 404.
// @Tags assignments
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userId path string true "ID сотрудника"
// @Success 200 {object} dto.ListUserAssignmentsResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/users/{userId}/assignments [get]
func (h *Handler) ListUserAssignments(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	userID, err := h.GetPathUUIDValue(c, pathKeyUserID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var resp dto.ListUserAssignmentsResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		items, listErr := services.Assignment.ListForUser(ctx, accountID, claims.UserID, userID)
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
