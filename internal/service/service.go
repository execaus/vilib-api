package service

import (
	"context"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/s3"

	"github.com/google/uuid"
)

type Auth interface {
	GenerateToken(userID uuid.UUID, accounts []uuid.UUID, currentAccountID uuid.UUID) (string, error)
	ComparePassword(hashedPassword string, password string) bool
	HashPassword(password string) (string, error)
	GeneratePassword() (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetClaimsFromToken(token string) (*domain.AuthClaims, error)
}

type Account interface {
	IsExistsUserByEmail(ctx context.Context, accountID uuid.UUID, email string) (bool, error)
	GetByUserEmail(ctx context.Context, email string) ([]domain.Account, error)
	GetByID(ctx context.Context, accountsID ...uuid.UUID) ([]domain.Account, error)
	Create(ctx context.Context, userName, userSurname, email string) (domain.Account, error)
	CreateUser(ctx context.Context, accountID uuid.UUID, name, surname, email string) (domain.User, error)
}

type AccountRole interface {
	Create(
		ctx context.Context,
		accountID uuid.UUID, name string, parentID *uuid.UUID, permission domain.PermissionMask, isDefault bool,
	) ([]domain.AccountRole, error)
}

type User interface {
	Create(ctx context.Context, name, surname, email, password string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) ([]domain.User, error)
	Update(ctx context.Context, initiatorID, targetID uuid.UUID, roleID *uuid.UUID) (domain.User, error)
}

type Email interface {
	SendRegisteredMail(ctx context.Context, email, password string) error
	SendCreateUserEmail(ctx context.Context, email, password string) error
}

type UserGroup interface {
	Create(ctx context.Context, accountID, initiatorID uuid.UUID, name string) (domain.UserGroup, error)
	AddMembers(
		ctx context.Context,
		accountID, initiatorID, groupID uuid.UUID,
		targetsID ...uuid.UUID,
	) ([]domain.GroupMember, error)
}

type GroupRole interface {
	Create(
		ctx context.Context,
		accountID uuid.UUID,
		name string,
		mask domain.PermissionMask,
		isDefault bool,
	) (domain.GroupRole, error)
}

type Video interface {
	GetPreflightUploadURL(ctx context.Context, accountID, groupID, userID uuid.UUID) (domain.PreflightURL, error)
	Update(ctx context.Context, videoID uuid.UUID, status *domain.VideoStatus) (domain.Video, error)
	Get(
		ctx context.Context,
		accountID, groupID, initiatorID, videoID uuid.UUID,
		isPreferOriginal bool,
	) (domain.PreflightURL, error)
}

type VideoAsset interface {
	Create(
		ctx context.Context,
		videoID uuid.UUID,
		tag domain.VideoAssetTag,
		bucketName, contentType string,
		bytes int,
	) (domain.VideoAsset, error)
	Get(ctx context.Context, videoID uuid.UUID) ([]domain.VideoAsset, error)
}

//go:generate mockgen -source=./service.go -destination=./mocks/service.go -package=mock_service
type Service struct {
	Auth
	Account
	User
	Email
	AccountRole
	UserGroup
	GroupRole
	Video
	VideoAsset
}

func NewService(cfg config.Config, localMailBox chan string, s3 s3.S3, r *repository.Repository) *Service {
	s := &Service{}

	s.Auth = NewAuthService(cfg.Auth, s)
	s.Account = NewAccountService(r.Account, s)
	s.User = NewUserService(r.User, s)
	s.Email = NewEmailService(cfg.Email, cfg.Server.Mode, localMailBox)
	s.AccountRole = NewAccountRoleService(r.AccountRole, s)
	s.UserGroup = NewUserGroupService(r.UserGroup, s)
	s.GroupRole = NewGroupRoleService(r.GroupRole, s)
	s.Video = NewVideoService(s3, r.Video, s)
	s.VideoAsset = NewVideoAssetService(r.VideoAsset, s)

	return s
}
