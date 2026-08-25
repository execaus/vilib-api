package handler

import (
	"context"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DeleteChapter godoc
// @Summary Удаление главы видео
// @Description Удаляет главу видео; соседние главы автоматически «наследуют» освободившийся
// @Description промежуток при следующем чтении границ (§1 дизайна эпика Э4). Доступно только
// @Description обладателю права ManageVideo.
// @Tags chapters
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Param chapterId path string true "ID главы"
// @Success 204
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId}/chapters/{chapterId} [delete]
func (h *Handler) DeleteChapter(c *gin.Context) {
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

	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		if err = services.Chapter.Delete(ctx, accountID, groupID, claims.UserID, videoID, chapterID); err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendNoContent(c)
}
