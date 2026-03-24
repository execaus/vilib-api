package service

import (
	"context"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
)

type Auth interface {
	GenerateToken(userID string, accounts []string, currentAccountID string) (string, error)
	ComparePassword(hashedPassword string, password string) bool
	HashPassword(password string) (string, error)
	GeneratePassword() (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetClaimsFromToken(token string) (*domain.AuthClaims, error)
}

type Account interface {
	IsExistsUserByEmail(ctx context.Context, accountID, email string) (bool, error)
	GetByUserEmail(ctx context.Context, email string) ([]domain.Account, error)
	GetByID(ctx context.Context, accountsID ...string) ([]domain.Account, error)
	Create(ctx context.Context, userName, userSurname, email string) (domain.Account, error)
	CreateUser(ctx context.Context, accountID, name, surname, email string) (domain.User, error)
}

type AccountRole interface {
	Create(
		ctx context.Context,
		accountID, name string,
		parentID *string,
		permission domain.PermissionMask,
		isDefault bool,
	) ([]domain.AccountRole, error)
}

type User interface {
	Create(ctx context.Context, name, surname, email, password string) (domain.User, error)
	GetByEmail(ctx context.Context, email string) ([]domain.User, error)
	Update(ctx context.Context, initiatorID, targetUserID string, status *domain.PermissionFlag) (domain.User, error)
}

type Email interface {
	SendRegisteredMail(ctx context.Context, email, password string) error
	SendCreateUserEmail(ctx context.Context, email, password string) error
}

//go:generate mockgen -source=./service.go -destination=./mocks/service.go -package=mock_service
type Service struct {
	Auth
	Account
	User
	Email
	AccountRole
}

func NewService(cfg config.Config, localMailBox chan string, r *repository.Repository) *Service {
	s := &Service{}

	s.Auth = NewAuthService(cfg.Auth, s)
	s.Account = NewAccountService(r.Account, s)
	s.User = NewUserService(r.User, s)
	s.Email = NewEmailService(cfg.Email, cfg.Server.Mode, localMailBox)
	s.AccountRole = NewAccountRoleService(r.AccountRole, s)

	return s
}
