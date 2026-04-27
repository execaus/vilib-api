package handler

import (
	"errors"
	"net/http"
	"vilib-api/internal/dto"
	"vilib-api/internal/repository"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func sendServiceError(c *gin.Context, err error) {
	var conflictError service.ConflictError
	var forbiddenError *service.ForbiddenError

	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, service.ErrNotFound) {
		sendNotFound(c, ErrCodeNotFound)
		return
	}

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

func sendServiceErrorWithDeactivated(c *gin.Context, err error) {
	if errors.Is(err, service.ErrUserDeactivated) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorMessage{Message: ErrCodeUserDeactivated})
		return
	}
	sendServiceError(c, err)
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

func sendNotFound(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotFound, &dto.ErrorMessage{Message: message})
}

func sendNoContent(c *gin.Context) {
	c.AbortWithStatus(http.StatusNoContent)
}
