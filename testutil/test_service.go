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
	setupMocks func(mockServices *ServiceMock, mockRepos *RepositoryMock),
	fn func(s *service.Service, r *repository.Repository),
) {
	mc := minimock.NewController(t)

	s := &ServiceMock{
		Auth:        mock_service.NewAuthMock(mc),
		User:        mock_service.NewUserMock(mc),
		Account:     mock_service.NewAccountMock(mc),
		Email:       mock_service.NewEmailMock(mc),
		AccountRole: mock_service.NewAccountRoleMock(mc),
		UserGroup:   mock_service.NewUserGroupMock(mc),
		GroupMember: mock_service.NewGroupMemberMock(mc),
		GroupRole:   mock_service.NewGroupRoleMock(mc),
		Video:       mock_service.NewVideoMock(mc),
		VideoAsset:  mock_service.NewVideoAssetMock(mc),
		Access:      mock_service.NewAccessMock(mc),
		S3:          mock_service.NewS3Mock(mc),
	}

	r := &RepositoryMock{
		Account:     mock_repository.NewAccountMock(mc),
		User:        mock_repository.NewUserMock(mc),
		AccountRole: mock_repository.NewAccountRoleMock(mc),
		UserGroup:   mock_repository.NewUserGroupMock(mc),
		GroupRole:   mock_repository.NewGroupRoleMock(mc),
		Video:       mock_repository.NewVideoMock(mc),
		VideoAsset:  mock_repository.NewVideoAssetMock(mc),
		GroupMember: mock_repository.NewGroupMemberMock(mc),
	}

	setupMocks(s, r)

	fn(s.ToServices(), r.ToRepositories())
}
