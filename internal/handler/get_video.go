package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetVideo godoc
// @Summary Получение точки доступа к видео
// @Description Возвращает точку доступа к видео по статусу (§4.4 дизайна эпика): для готового
// @Description видео без is_prefer_original — URL мастер-плейлиста HLS с HLS-токеном в query,
// @Description иначе — преподписанный URL на оригинал. Возвращает 409, если видео недоступно
// @Description (ещё загружается или обработка завершилась ошибкой без сохранённого оригинала).
// @Tags videos
// @Accept json
// @Produce json
// @Param accountId path string true "ID аккаунта"
// @Param groupId path string true "ID группы пользователей"
// @Param videoId path string true "ID видео"
// @Param is_prefer_original query bool false "Предпочитать оригинальное видео"
// @Success 200 {object} dto.GetVideoResponse
// @Failure 400 {object} dto.ErrorMessage
// @Failure 401
// @Failure 403 {object} dto.ErrorMessage
// @Failure 404 {object} dto.ErrorMessage
// @Failure 409 {object} dto.ErrorMessage
// @Failure 500
// @Router /api/v1/accounts/{accountId}/user-groups/{groupId}/video/{videoId} [get]
func (h *Handler) GetVideo(c *gin.Context) {
	var query dto.GetVideoQuery

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

	if err = c.ShouldBindQuery(&query); err != nil {
		sendBadRequest(c, err)
		return
	}

	var access domain.VideoAccess
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		access, err = services.Video.Get(ctx, accountID, groupID, claims.UserID, videoID, query.IsPreferOriginal)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		return nil
	}); err != nil {
		sendServiceError(c, err)
		return
	}

	sendOK(c, dto.GetVideoResponse{
		Kind:       string(access.Kind),
		URL:        h.videoAccessURL(c, access, accountID, groupID, videoID),
		ExpiresAt:  access.ExpiresAt,
		Status:     uint(access.Video.Status),
		StatusName: access.Video.Status.String(),
		Profiles:   access.Profiles,
	})
}

// videoAccessURL собирает итоговый URL точки доступа к видео: для оригинала — преподписанный
// URL как есть, для HLS — абсолютный URL мастер-плейлиста с HLS-токеном в query (сервису
// неизвестен публичный адрес API, поэтому URL собирает handler). Схема и хост берутся из
// текущего запроса (Host, X-Forwarded-Proto/TLS) — этого достаточно для локального стенда
// без выделенного публичного домена.
func (h *Handler) videoAccessURL(
	c *gin.Context,
	access domain.VideoAccess,
	accountID, groupID, videoID uuid.UUID,
) string {
	if access.Kind != domain.VideoAccessKindHLS {
		return string(access.URL)
	}

	path := GetVideoHLSMasterURL.WithValues(accountID.String(), groupID.String(), videoID.String())

	return requestOrigin(c) + "/api/v1/" + path + "?token=" + access.HLSToken
}

// requestOrigin определяет схему и хост текущего запроса для формирования абсолютных URL ответа.
func requestOrigin(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	return scheme + "://" + c.Request.Host
}
