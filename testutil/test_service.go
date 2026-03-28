package testutil

import (
	"testing"
	"vilib-api/internal/repository"
	mock_repository "vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	mock_service "vilib-api/internal/service/service_mocks"

	"github.com/gojuno/minimock/v3"
)

func TestService(
	t *testing.T,
	prepareFn func(mockServices *ServiceMock, mockRepos *RepositoryMock),
	fn func(s *service.Service, r *repository.Repository),
) {
	ctrl := minimock.NewController(t)

	s := &ServiceMock{
		Auth:        mock_service.NewAuthMock(ctrl),
		User:        mock_service.NewUserMock(ctrl),
		Account:     mock_service.NewAccountMock(ctrl),
		Email:       mock_service.NewEmailMock(ctrl),
		AccountRole: mock_service.NewAccountRoleMock(ctrl),
		UserGroup:   mock_service.NewUserGroupMock(ctrl),
		GroupRole:   mock_service.NewGroupRoleMock(ctrl),
		Video:       mock_service.NewVideoMock(ctrl),
		VideoAsset:  mock_service.NewVideoAssetMock(ctrl),
	}

	r := &RepositoryMock{
		Account:     mock_repository.NewAccountMock(ctrl),
		User:        mock_repository.NewUserMock(ctrl),
		AccountRole: mock_repository.NewAccountRoleMock(ctrl),
		UserGroup:   mock_repository.NewUserGroupMock(ctrl),
		GroupRole:   mock_repository.NewGroupRoleMock(ctrl),
		Video:       mock_repository.NewVideoMock(ctrl),
		VideoAsset:  mock_repository.NewVideoAssetMock(ctrl),
	}

	prepareFn(s, r)

	fn(s.ToServices(), r.ToRepositories())
}
