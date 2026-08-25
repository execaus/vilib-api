package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UpdateChapter godoc
// @Summary Редактирование главы видео
// @Description Меняет начало и/или название главы; конец главы пересчитывается сервером.
// @Description Правка допустима и после того, как видео уже смотрели — статусы глав
// @Description пересчитываются по текущей разметке, зачёт видео не меняется (Э4-Т7). Доступно
// @Description только обладателю права ManageVideo.
// @Tags chapters
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Param chapterId path string true "ID главы"
// @Param request body dto.UpdateChapterRequest true "Тело запроса"
// @Success 200 {object} dto.ChapterResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId}/chapters/{chapterId} [patch]
func (h *Handler) UpdateChapter(c *gin.Context) {
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

	chapterID, err := h.GetPathUUIDValue(c, pathKeyChapterID)
	if err != nil {
		sendBadRequest(c, err)
		return
	}

	var req dto.UpdateChapterRequest
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

		bound, updateErr := services.Chapter.Update(
			ctx, accountID, groupID, claims.UserID, videoID, chapterID, req.ToDomain(),
		)
		if updateErr != nil {
			zap.L().Error(updateErr.Error())
			return updateErr
		}

		resp.Chapter.FromDomainBound(bound)

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
