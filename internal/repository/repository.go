package repository

import (
	"context"
	"vilib-api/internal/domain"
)

type Account interface {
	Insert(ctx context.Context, name, email string) (domain.Account, error)
	SelectByUsersID(ctx context.Context, id ...string) ([]domain.Account, error)
}

type User interface {
	SelectByEmail(ctx context.Context, email string) ([]domain.User, error)
	Insert(ctx context.Context, name, surname, hash, email string) (domain.User, error)
}

type AccountStatus interface {
	Upsert(ctx context.Context, userID, accountID string, value domain.BitmapValue) error
}

type Repository struct {
	Account
	User
	AccountStatus
}

func NewRepository(provider *ExecutorProvider) *Repository {
	return &Repository{
		Account:       NewAccountRepository(provider),
		User:          NewUserRepository(provider),
		AccountStatus: NewAccountStatusRepository(provider),
	}
}
