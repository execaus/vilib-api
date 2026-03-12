package handler

import (
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	RegisterURL = "auth/register"
	APIVersion1 = "v1"
)

type Handler struct {
	saga         service.SagaRunner
	localMailBox chan string
}

func NewHandler(saga service.SagaRunner, localMailBox chan string) *Handler {
	h := &Handler{
		saga:         saga,
		localMailBox: localMailBox,
	}

	return h
}

func (h *Handler) GetRouter() *gin.Engine {
	engine := gin.Default()

	api := engine.Group("api")

	v1 := api.Group(APIVersion1)

	v1.POST(RegisterURL, h.Register)

	return engine
}

func (h *Handler) LocalMailBox() <-chan string {
	return h.localMailBox
}
