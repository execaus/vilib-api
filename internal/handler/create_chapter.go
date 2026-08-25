package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CreateChapter godoc
// @Summary Создание главы видео
// @Description Создаёт главу видео по моменту начала — конец главы вычисляется сервером как
// @Description начало следующей главы либо длительность видео (§1, §4, §5 дизайна эпика Э4).
// @Description Доступно только обладателю права ManageVideo и только у видео в статусе ready.
// @Tags chapters
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Param request body dto.CreateChapterRequest true "Тело запроса"
// @Success 201 {object} dto.ChapterResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId}/chapters [post]
func (h *Handler) CreateChapter(c *gin.Context) {
	accountID, err := h.GetPathUUIDValue(c, pathKeyAccountID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	groupID, err := h.GetPathUUIDValue(c, pathKeyUserGroupID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	videoID, err := h.GetPathUUIDValue(c, pathKeyVideoID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var req dto.CreateChapterRequest
	if err = c.BindJSON(&req); err != nil {
		sendBadRequest(c, err)
		return
	}

	var resp dto.ChapterResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		bound, createErr := services.Chapter.Create(
			ctx, accountID, groupID, claims.UserID, videoID,
			domain.CreateChapter{StartMs: req.StartMs, Name: req.Name},
		)
		if createErr != nil {
			zap.L().Error(createErr.Error())
			return createErr
		}

		resp.Chapter.FromDomainBound(bound)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendCreated(c, resp)
}
