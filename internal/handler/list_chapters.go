package handler

import (
	"context"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ListChapters godoc
// @Summary Список глав видео
// @Description Возвращает главы видео, упорядоченные по времени начала, вместе с покрытием
// @Description просмотра инициатора (пройдена/частично/не просмотрена). Видео без глав отдаёт
// @Description пустой список, а не 404 (§4, §5 дизайна эпика Э4).
// @Tags chapters
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param userGroupId path string true "ID группы"
// @Param videoId path string true "ID видео"
// @Success 200 {object} dto.ListChaptersResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 500
// @Security BearerAuth
// @Router /api/v1/accounts/{accountId}/user-groups/{userGroupId}/video/{videoId}/chapters [get]
func (h *Handler) ListChapters(c *gin.Context) {
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

	var resp dto.ListChaptersResponse
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, claimsErr := h.claims(c)
		if claimsErr != nil {
			zap.L().Error(claimsErr.Error())
			return claimsErr
		}

		progress, listErr := services.Chapter.List(ctx, accountID, groupID, claims.UserID, videoID)
		if listErr != nil {
			zap.L().Error(listErr.Error())
			return listErr
		}

		resp.Chapters = make([]dto.Chapter, len(progress))
		for i, p := range progress {
			resp.Chapters[i].FromDomainProgress(p, h.deps.PublicConfig.CompletionThreshold)
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, resp)
}
