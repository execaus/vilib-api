package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CancelAssignment godoc
// @Summary Отмена назначения обязательного обучения
// @Description Отменяет назначение целиком (§4 дизайна эпика Э3): незавершённые участники
// @Description переводятся в cancelled, завершённые остаются с подтверждённым просмотром.
// @Description Повторная отмена — 409 conflict.assignment_cancelled.
// @Tags assignments
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param assignmentId path string true "ID назначения"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/assignments/{assignmentId} [delete]
func (h *Handler) CancelAssignment(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	assignmentID, err := h.GetPathUUIDValue(c, pathKeyAssignmentID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		if cancelErr := services.Assignment.Cancel(ctx, accountID, claims.UserID, assignmentID); cancelErr != nil {
			zap.L().Error(cancelErr.Error())
			return cancelErr
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendNoContent(c)
}
