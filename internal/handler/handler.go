package handler

import (
	"vilib-api/internal/pkg"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "vilib-api/docs"
)

const (
	pathKeyAccountID = iota
	pathKeyUserID
)

var (
	APIVersion1       = "v1"
	RegisterURL       = "auth/register"
	LoginURL          = "auth/login"
	CreateUserURL     = pkg.NewURLSupplier("accounts/%s/users")
	CreateRoleAccount = pkg.NewURLSupplier("accounts/%s/roles")
	UpdateUserURL     = pkg.NewURLSupplier("users/%s")
)

type Handler struct {
	saga saga.Runner[*service.Service]
}

func NewHandler(saga saga.Runner[*service.Service]) *Handler {
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
	v1.POST(CreateUserURL.WithTemplateParams(pathKeyAccountID), h.CreateUser)
	v1.POST(CreateRoleAccount.WithTemplateParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateAccountRole)
	v1.PUT(UpdateUserURL.WithTemplateParams(pathKeyUserID), h.RequireAuthMiddleware, h.UpdateUser)

	return engine
}
