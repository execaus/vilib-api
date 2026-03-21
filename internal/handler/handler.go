package handler

import (
	"fmt"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "vilib-api/docs"
)

const (
	pathKeyAccountID = iota
)

var (
	APIVersion1   = "v1"
	RegisterURL   = "auth/register"
	LoginURL      = "auth/login"
	CreateUserURL = fmt.Sprintf("accounts/{%s}/users", pathKeyAccountID)
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

// @title Vilib API
// @version 1.0
// @description API для управления внутренней видео документацией Vilib.
// @host localhost:8080
// @BasePath /api/v1
func (h *Handler) GetRouter() *gin.Engine {
	engine := gin.Default()

	api := engine.Group("api")

	api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := api.Group(APIVersion1)

	v1.POST(RegisterURL, h.Register)
	v1.POST(LoginURL, h.Login)
	v1.POST(CreateUserURL, h.CreateUser)

	// Добавление пользователей к аккаунту
	// Назначение ролей

	return engine
}
