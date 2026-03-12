package handler

import (
	"errors"
	"net/http"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ErrorMessage struct {
	Message string `json:"message"`
}

func sendServiceError(c *gin.Context, err error) {
	var serviceError *service.ConflictError

	if errors.As(err, &serviceError) {
		sendConflict(c, serviceError.Error())
		return
	}

	sendInternalError(c, err)
}

func sendBadRequest(c *gin.Context, err error) {
	c.AbortWithStatusJSON(http.StatusBadRequest, err.Error())
}

func sendInternalError(c *gin.Context, err error) {
	zap.L().Error(err.Error())
	c.AbortWithStatus(http.StatusInternalServerError)
}

func sendConflict(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusConflict, &ErrorMessage{Message: message})
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
