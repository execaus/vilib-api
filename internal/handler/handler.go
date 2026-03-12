package handler

import (
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	saga service.SagaRunner
}

func NewHandler(saga service.SagaRunner) *Handler {
	h := &Handler{
		saga: saga,
	}

	return h
}

func (h *Handler) GetRouter() *gin.Engine {
	engine := gin.Default()

	return engine
}
