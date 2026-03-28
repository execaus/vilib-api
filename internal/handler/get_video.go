package handler

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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

	var (
		preflightURL domain.PreflightURL
	)
	if err = h.saga.Run(c, func(ctx context.Context, services *service.Service) error {
		claims, err := h.getClaims(c, services.Auth)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		preflightURL, err = services.Video.Get(ctx, accountID, groupID, claims.UserID, videoID, query.IsPreferOriginal)
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
		PresignedURL: preflightURL,
	})
}
