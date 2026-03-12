package handler

import (
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	RegisterURL = "auth/register"
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

	api := engine.Group("api")

	v1 := api.Group("v1")

	v1.POST(RegisterURL, h.Register)

	return engine
}
