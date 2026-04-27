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
	pathKeyRoleID
	pathKeyGroupRoleID
	pathKeyGroupMemberUserID
)

var (
	APIVersion1          = "v1"
	RegisterURL          = "auth/register"
	LoginURL             = "auth/login"
	CreateUserURL        = NewURLSupplier("accounts/%s/users")
	CreateAccountRoleURL = NewURLSupplier("accounts/%s/roles")
	UpdateUserURL        = NewURLSupplier("accounts/%s/users/%s")
	CreateUserGroupURL   = NewURLSupplier("accounts/%s/user-groups")
	AddGroupMemberURL    = NewURLSupplier("accounts/%s/user-groups/%s/members")
	CreateGroupRoleURL   = NewURLSupplier("accounts/%s/user-groups/roles")
	UploadVideoUrl       = NewURLSupplier("accounts/%s/user-groups/%s/video")
	GetVideoUrl          = NewURLSupplier("accounts/%s/user-groups/%s/video/%s")

	ListUsersURL         = NewURLSupplier("accounts/%s/users")
	ReactivateUserURL    = NewURLSupplier("accounts/%s/users/%s/reactivate")
	ListAccountRolesURL  = NewURLSupplier("accounts/%s/roles")
	DeleteAccountRoleURL = NewURLSupplier("accounts/%s/roles/%s")
	ListUserGroupsURL    = NewURLSupplier("accounts/%s/user-groups")
	DeleteUserGroupURL   = NewURLSupplier("accounts/%s/user-groups/%s")
	DeleteGroupMemberURL = NewURLSupplier("accounts/%s/user-groups/%s/members/%s")
	ListGroupRolesURL    = NewURLSupplier("accounts/%s/user-groups/roles")
	DeleteGroupRoleURL   = NewURLSupplier("accounts/%s/user-groups/roles/%s")
	ListVideosURL        = NewURLSupplier("accounts/%s/user-groups/%s/videos")
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

	// Users
	v1.POST(CreateUserURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateUser)
	v1.GET(ListUsersURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.GetAllUsers)
	v1.PUT(UpdateUserURL.WithPathParams(pathKeyAccountID, pathKeyUserID), h.RequireAuthMiddleware, h.UpdateUser)
	v1.DELETE(UpdateUserURL.WithPathParams(pathKeyAccountID, pathKeyUserID), h.RequireAuthMiddleware, h.DeactivateUser)
	v1.POST(ReactivateUserURL.WithPathParams(pathKeyAccountID, pathKeyUserID), h.RequireAuthMiddleware, h.ReactivateUser)

	// Account roles
	v1.POST(CreateAccountRoleURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateAccountRole)
	v1.GET(ListAccountRolesURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.GetAllAccountRoles)
	v1.DELETE(DeleteAccountRoleURL.WithPathParams(pathKeyAccountID, pathKeyRoleID), h.RequireAuthMiddleware, h.DeleteAccountRole)

	// User groups
	v1.POST(CreateUserGroupURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateUserGroup)
	v1.GET(ListUserGroupsURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.GetAllUserGroups)
	v1.DELETE(DeleteUserGroupURL.WithPathParams(pathKeyAccountID, pathKeyUserGroupID), h.RequireAuthMiddleware, h.DeleteUserGroup)

	// Group members
	v1.POST(
		AddGroupMemberURL.WithPathParams(pathKeyAccountID, pathKeyUserGroupID),
		h.RequireAuthMiddleware,
		h.AddGroupMember,
	)
	v1.DELETE(
		DeleteGroupMemberURL.WithPathParams(pathKeyAccountID, pathKeyUserGroupID, pathKeyGroupMemberUserID),
		h.RequireAuthMiddleware,
		h.DeleteGroupMember,
	)

	// Group roles
	v1.POST(CreateGroupRoleURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.CreateGroupRole)
	v1.GET(ListGroupRolesURL.WithPathParams(pathKeyAccountID), h.RequireAuthMiddleware, h.GetAllGroupRoles)
	v1.DELETE(DeleteGroupRoleURL.WithPathParams(pathKeyAccountID, pathKeyGroupRoleID), h.RequireAuthMiddleware, h.DeleteGroupRole)

	// Videos
	v1.POST(
		UploadVideoUrl.WithPathParams(pathKeyAccountID, pathKeyUserGroupID),
		h.RequireAuthMiddleware,
		h.UploadVideo,
	)
	v1.GET(
		ListVideosURL.WithPathParams(pathKeyAccountID, pathKeyUserGroupID),
		h.RequireAuthMiddleware,
		h.GetAllVideos,
	)
	v1.GET(
		GetVideoUrl.WithPathParams(pathKeyAccountID, pathKeyUserGroupID, pathKeyVideoID),
		h.RequireAuthMiddleware,
		h.GetVideo,
	)
	v1.PUT(
		GetVideoUrl.WithPathParams(pathKeyAccountID, pathKeyUserGroupID, pathKeyVideoID),
		h.RequireAuthMiddleware,
		h.RenameVideo,
	)
	v1.DELETE(
		GetVideoUrl.WithPathParams(pathKeyAccountID, pathKeyUserGroupID, pathKeyVideoID),
		h.RequireAuthMiddleware,
		h.DeleteVideo,
	)

	return engine
}
