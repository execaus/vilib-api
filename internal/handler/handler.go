package handler

import (
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "vilib-api/docs"
)

type (
	PathKey uint
)

const (
	pathKeyAccountID PathKey = iota
	pathKeyUserID
	pathKeyUserGroupID
	pathKeyVideoID
)

var (
	APIVersion1          = "v1"
	RegisterURL          = "auth/register"
	LoginURL             = "auth/login"
	CreateUserURL        = NewURLSupplier("accounts/%s/users")
	CreateAccountRoleURL = NewURLSupplier("accounts/%s/roles")
	UpdateUserURL        = NewURLSupplier("users/%s")
	CreateUserGroupURL   = NewURLSupplier("accounts/%s/user-groups")
	AddGroupMemberURL    = NewURLSupplier("accounts/%s/user-groups/%s/members")
	CreateGroupRoleURL   = NewURLSupplier("accounts/%s/user-groups/roles")
	UploadVideoUrl       = NewURLSupplier("accounts/%s/user-groups/%s/video")
	GetVideoUrl          = NewURLSupplier("accounts/%s/user-groups/%s/video/%s")
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
	v1.POST(CreateUserURL.WithPathParams(pathKeyAccountID), h.CreateUser)
	v1.POST(CreateAccountRoleURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateAccountRole)
	v1.PUT(UpdateUserURL.WithPathParams(pathKeyUserID), h.RequireAuthMiddleware, h.UpdateUser)
	v1.POST(CreateUserGroupURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateUserGroup)
	v1.POST(
		AddGroupMemberURL.WithPathParams(pathKeyAccountID, pathKeyUserGroupID),
		h.RequireAuthMiddleware,
		h.AddGroupMember,
	)
	v1.POST(CreateGroupRoleURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateGroupRole)
	v1.POST(
		UploadVideoUrl.WithPathParams(pathKeyAccountID, pathKeyUserGroupID),
		h.RequireAuthMiddleware,
		h.UploadVideo,
	)
	v1.GET(
		GetVideoUrl.WithPathParams(pathKeyAccountID, pathKeyUserGroupID, pathKeyVideoID),
		h.RequireAuthMiddleware,
		h.GetVideo,
	)

	return engine
}
