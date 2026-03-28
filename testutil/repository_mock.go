package testutil

import (
	"vilib-api/internal/repository"
	mock_repository "vilib-api/internal/repository/repository_mocks"
)

type RepositoryMock struct {
	Account     *mock_repository.AccountMock
	User        *mock_repository.UserMock
	AccountRole *mock_repository.AccountRoleMock
	UserGroup   *mock_repository.UserGroupMock
	GroupRole   *mock_repository.GroupRoleMock
	Video       *mock_repository.VideoMock
	VideoAsset  *mock_repository.VideoAssetMock
}

func (r *RepositoryMock) ToRepositories() *repository.Repository {
	return &repository.Repository{
		Account:     r.Account,
		User:        r.User,
		AccountRole: r.AccountRole,
		UserGroup:   r.UserGroup,
		GroupRole:   r.GroupRole,
		Video:       r.Video,
		VideoAsset:  r.VideoAsset,
	}
}
