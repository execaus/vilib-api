package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DeleteAssignmentParticipant godoc
// @Summary Снятие участника с назначения
// @Description Снимает одного сотрудника с назначения (§4 дизайна эпика Э3): участие
// @Description переводится в cancelled с причиной removed_by_manager. Уже подтвердившего
// @Description просмотр снять нельзя — 409 conflict.participant_completed.
// @Tags assignments
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param assignmentId path string true "ID назначения"
// @Param userId path string true "ID участника"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/assignments/{assignmentId}/participants/{userId} [delete]
func (h *Handler) DeleteAssignmentParticipant(c *gin.Context) {
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

	userID, err := h.GetPathUUIDValue(c, pathKeyUserID)
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

		removeErr := services.Assignment.RemoveParticipant(ctx, accountID, claims.UserID, assignmentID, userID)
		if removeErr != nil {
			zap.L().Error(removeErr.Error())
			return removeErr
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendNoContent(c)
}
