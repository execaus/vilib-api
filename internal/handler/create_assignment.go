package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreateAssignment godoc
// @Summary Назначение обязательного обучения
// @Description Назначает видео пользователям и/или группе со сроком и комментарием (§4
// @Description дизайна эпика Э3): раскрывает цели в персональные записи, отклонённые (нет
// @Description доступа/деактивирован/не в аккаунте) возвращаются в rejected, а не ошибкой.
// @Description Цель-группа допустима только для группы, которой принадлежит видео.
// @Tags assignments
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param request body dto.CreateAssignmentRequest true "Тело запроса"
// @Success 201 {object} dto.CreateAssignmentResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/assignments [post]
func (h *Handler) CreateAssignment(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var req dto.CreateAssignmentRequest
	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var resp dto.CreateAssignmentResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		details, rejected, createErr := services.Assignment.Create(ctx, accountID, claims.UserID, req.ToDomain())
		if createErr != nil {
			zap.L().Error(createErr.Error())
			return createErr
		}

		resp.Assignment.FromDomain(details)
		resp.Rejected = make([]dto.RejectedTarget, len(rejected))
		for i, r := range rejected {
			resp.Rejected[i].FromDomain(r)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendCreated(c, resp)
}
