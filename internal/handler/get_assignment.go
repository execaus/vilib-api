package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetAssignment godoc
// @Summary Карточка назначения обязательного обучения
// @Description Возвращает назначение целиком: цели, счётчики, участников с покрытием и
// @Description признаком доступа к видео, журнал изменений (§4 дизайна эпика Э3). Право
// @Description чтения — назначивший видит своё назначение всегда, иначе — обладатель права
// @Description назначения в области (В-8).
// @Tags assignments
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param assignmentId path string true "ID назначения"
// @Success 200 {object} dto.GetAssignmentResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/assignments/{assignmentId} [get]
func (h *Handler) GetAssignment(c *gin.Context) {
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

	var resp dto.GetAssignmentResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		details, getErr := services.Assignment.Get(ctx, accountID, claims.UserID, assignmentID)
		if getErr != nil {
			zap.L().Error(getErr.Error())
			return getErr
		}

		resp.Assignment.FromDomain(details)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
