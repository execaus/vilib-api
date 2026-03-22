package testutil

import (
	"vilib-api/internal/service"
	mock_service "vilib-api/internal/service/mocks"
)

type ServiceMock struct {
	Auth          *mock_service.MockAuth
	User          *mock_service.MockUser
	Account       *mock_service.MockAccount
	Email         *mock_service.MockEmail
	AccountStatus *mock_service.MockAccountStatus
}

func (s *ServiceMock) ToServices() *service.Service {
	return &service.Service{
		Auth:          s.Auth,
		Account:       s.Account,
		User:          s.User,
		Email:         s.Email,
		AccountStatus: s.AccountStatus,
	}
}
