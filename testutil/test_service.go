package testutil

import (
	"testing"
	"vilib-api/internal/repository"
	mock_repository "vilib-api/internal/repository/mocks"
	"vilib-api/internal/service"
	mock_service "vilib-api/internal/service/mocks"

	"go.uber.org/mock/gomock"
)

func TestService(
	t *testing.T,
	prepareFn func(mockServices *ServiceMock, mockRepos *RepositoryMock),
	fn func(s *service.Service, r *repository.Repository),
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &ServiceMock{
		Auth:          mock_service.NewMockAuth(ctrl),
		Account:       mock_service.NewMockAccount(ctrl),
		User:          mock_service.NewMockUser(ctrl),
		Email:         mock_service.NewMockEmail(ctrl),
		AccountStatus: mock_service.NewMockAccountStatus(ctrl),
	}

	r := &RepositoryMock{
		Account:       mock_repository.NewMockAccount(ctrl),
		User:          mock_repository.NewMockUser(ctrl),
		AccountStatus: mock_repository.NewMockAccountStatus(ctrl),
	}

	prepareFn(s, r)

	fn(s.ToServices(), r.ToRepositories())
}
