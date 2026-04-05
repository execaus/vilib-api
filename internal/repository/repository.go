package repository

import (
	"context"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

//go:generate minimock -i Account -o ./repository_mocks/account_mock.go
//go:generate minimock -i User -o ./repository_mocks/user_mock.go
//go:generate minimock -i AccountRole -o ./repository_mocks/account_role_mock.go
//go:generate minimock -i UserGroup -o ./repository_mocks/user_group_mock.go
//go:generate minimock -i GroupRole -o ./repository_mocks/group_role_mock.go
//go:generate minimock -i Video -o ./repository_mocks/video_mock.go
//go:generate minimock -i VideoAsset -o ./repository_mocks/video_asset_mock.go

type Account interface {
	Insert(ctx context.Context, name, email string) (domain.Account, error)
	SelectByUsersID(ctx context.Context, usersID ...uuid.UUID) ([]domain.Account, error)
	SelectByID(ctx context.Context, accountsID ...uuid.UUID) ([]domain.Account, error)
}

type User interface {
	SelectByEmail(ctx context.Context, email string) ([]domain.User, error)
	Insert(ctx context.Context, name, surname, hash, email string, roleID uuid.UUID) (domain.User, error)
	SelectByID(ctx context.Context, usersID ...uuid.UUID) ([]domain.User, error)
}

type AccountRole interface {
	Insert(
		ctx context.Context,
		accountID uuid.UUID, name string, parentID *uuid.UUID, permission domain.PermissionMask, isDefault, isSystem bool,
	) (domain.AccountRole, error)
	SelectByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.AccountRole, error)
	SelectByID(ctx context.Context, rolesID ...uuid.UUID) ([]domain.AccountRole, error)
}

type UserGroup interface {
	Insert(ctx context.Context, accountID uuid.UUID, name string) (domain.UserGroup, error)
}

type GroupMember interface {
	Insert(ctx context.Context, groupID, roleID uuid.UUID, usersID ...uuid.UUID) ([]domain.GroupMember, error)
}

type GroupRole interface {
	Insert(
		ctx context.Context,
		accountID uuid.UUID,
		name string,
		permission domain.PermissionMask,
		isDefault bool,
	) (domain.GroupRole, error)
	SelectByAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GroupRole, error)
}

type Video interface {
	Select(ctx context.Context, id uuid.UUID) (domain.Video, error)
	Insert(ctx context.Context, name string, groupID, userID uuid.UUID, status domain.VideoStatus) (domain.Video, error)
	Update(ctx context.Context, id uuid.UUID, status *domain.VideoStatus) (domain.Video, error)
}

type VideoAsset interface {
	Select(ctx context.Context, videoID uuid.UUID) ([]domain.VideoAsset, error)
	Create(
		ctx context.Context,
		videoID uuid.UUID,
		tag domain.VideoAssetTag,
		bucketName, contentType string,
		bytes int,
	) (domain.VideoAsset, error)
}

type Repository struct {
	Account
	User
	AccountRole
	UserGroup
	GroupMember
	GroupRole
	Video
	VideoAsset
}

func NewRepository(provider *ExecutorProvider) *Repository {
	return &Repository{
		Account:     NewAccountRepository(provider),
		User:        NewUserRepository(provider),
		AccountRole: NewAccountRoleRepository(provider),
		UserGroup:   NewUserGroupRepository(provider),
		GroupMember: NewGroupMemberRepository(provider),
		GroupRole:   NewGroupRoleRepository(provider),
	}
}
