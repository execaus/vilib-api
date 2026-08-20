package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateAssignment godoc
// @Summary Изменение назначения обязательного обучения
// @Description Меняет срок и/или комментарий назначения (§4 дизайна эпика Э3): новый срок
// @Description проверяется как при создании и пересчитывается всем незавершённым участникам,
// @Description завершённые записи не затрагиваются. Отменённое назначение не редактируется.
// @Tags assignments
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param assignmentId path string true "ID назначения"
// @Param request body dto.UpdateAssignmentRequest true "Тело запроса"
// @Success 200 {object} dto.UpdateAssignmentResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/assignments/{assignmentId} [patch]
func (h *Handler) UpdateAssignment(c *gin.Context) {
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

	var req dto.UpdateAssignmentRequest
	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var resp dto.UpdateAssignmentResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		details, updateErr := services.Assignment.UpdateDue(
			ctx, accountID, claims.UserID, assignmentID, req.ToDomain(),
		)
		if updateErr != nil {
			zap.L().Error(updateErr.Error())
			return updateErr
		}

		resp.Assignment.FromDomain(details)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
