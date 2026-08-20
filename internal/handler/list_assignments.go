package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ListAssignments godoc
// @Summary Список/отчёт по назначениям
// @Description Возвращает назначения аккаунта с фильтрами и счётчиками статусов (§4, §5, §6
// @Description дизайна эпика Э3, В-53). Область видимости — правило В-8: назначивший видит
// @Description свои назначения всегда, иначе — только область права (аккаунт целиком или
// @Description список групп); нет области — пустой список, не ошибка. include_deactivated=false
// @Description (по умолчанию) исключает деактивированных из счётчиков и участников.
// @Description expand_participants=true добавляет в каждый элемент список участников — основа
// @Description сводки «сотрудник × видео» (С-7). due_from/due_to задают период срока
// @Description (включительно, границы независимы); попадание в период проверяется по-разному
// @Description в зависимости от режима срока назначения (В-61): в режиме «дата» — по общему
// @Description сроку назначения, в режиме «N дней с зачисления» — по персональному сроку хотя
// @Description бы одного незавершённого участника (отменённые участия не учитываются).
// @Tags assignments
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param group_id query string false "ID группы видео"
// @Param video_id query string false "ID видео"
// @Param user_id query string false "ID участника"
// @Param status query string false "Статус назначения: active, cancelled"
// @Param due_from query string false "Начало периода срока, включительно (RFC3339)"
// @Param due_to query string false "Конец периода срока, включительно (RFC3339)"
// @Param include_deactivated query bool false "Включать деактивированных участников"
// @Param expand_participants query bool false "Добавить участников в каждый элемент"
// @Success 200 {object} dto.ListAssignmentsResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/assignments [get]
func (h *Handler) ListAssignments(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var query dto.AssignmentListQuery
	if err = c.ShouldBindQuery(&query); err != nil {
		sendBadRequest(c, err)
		return
	}

	filter, err := query.ToDomain()
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var resp dto.ListAssignmentsResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		items, listErr := services.Assignment.List(ctx, accountID, claims.UserID, filter)
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
