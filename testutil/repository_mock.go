package testutil

import (
	"vilib-api/internal/repository"
	mock_repository "vilib-api/internal/repository/mocks"
)

type RepositoryMock struct {
	Account       *mock_repository.MockAccount
	User          *mock_repository.MockUser
	AccountStatus *mock_repository.MockAccountStatus
}

func (r *RepositoryMock) ToRepositories() *repository.Repository {
	return &repository.Repository{
		Account:       r.Account,
		User:          r.User,
		AccountStatus: r.AccountStatus,
	}
}
