package handler

import (
	"errors"
	"net/http"
	"vilib-api/internal/dto"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func sendServiceError(c *gin.Context, err error) {
	var conflictError *service.ConflictError
	var forbiddenError *service.ForbiddenError

	if errors.As(err, &conflictError) {
		sendConflict(c, err.Error())
		return
	}

	if errors.As(err, &forbiddenError) {
		sendForbidden(c, err.Error())
		return
	}

	sendInternalError(c, err)
}

func sendBadRequest(c *gin.Context, err error) {
	c.AbortWithStatusJSON(http.StatusBadRequest, &dto.ErrorMessage{Message: err.Error()})
}

func sendInternalError(c *gin.Context, err error) {
	zap.L().Error(err.Error())
	c.AbortWithStatus(http.StatusInternalServerError)
}

func sendConflict(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusConflict, &dto.ErrorMessage{Message: message})
}

func sendCreated(c *gin.Context, body any) {
	c.AbortWithStatusJSON(http.StatusCreated, body)
}

func sendOK(c *gin.Context, body any) {
	c.AbortWithStatusJSON(http.StatusOK, body)
}

func sendUnauthorized(c *gin.Context) {
	c.AbortWithStatus(http.StatusUnauthorized)
}

func sendForbidden(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, &dto.ErrorMessage{Message: message})
}
