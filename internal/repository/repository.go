package repository

import (
	"context"
	"vilib-api/internal/domain"
)

type Account interface {
	Insert(ctx context.Context, name, email string) (domain.Account, error)
	SelectByUsersID(ctx context.Context, id ...string) ([]domain.Account, error)
	SelectByID(ctx context.Context, accountsID ...string) ([]domain.Account, error)
}

type User interface {
	SelectByEmail(ctx context.Context, email string) ([]domain.User, error)
	Insert(ctx context.Context, name, surname, hash, email string) (domain.User, error)
	SelectByID(ctx context.Context, usersID ...string) ([]domain.User, error)
}

type AccountRole interface {
	Insert(
		ctx context.Context,
		accountID, name string,
		parentID *string,
		permission domain.PermissionMask,
		isDefault bool,
	) (domain.AccountRole, error)
	SelectByAccountID(ctx context.Context, accountID string) ([]domain.AccountRole, error)
}

//go:generate mockgen -source=./repository.go -destination=./mocks/repository.go -package=mock_repository
type Repository struct {
	Account
	User
	AccountRole
}

func NewRepository(provider *ExecutorProvider) *Repository {
	return &Repository{
		Account:     NewAccountRepository(provider),
		User:        NewUserRepository(provider),
		AccountRole: NewAccountRoleRepository(provider),
	}
}
