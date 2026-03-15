package service

import (
	"context"
	"vilib-api/config"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"
)

type Auth interface {
	GenerateToken(ctx context.Context, accounts []string, userID, currentAccountID string) (string, error)
	GetClaimsFromToken(ctx context.Context, token string) (*models.AuthClaims, error)
	ComparePassword(ctx context.Context, hashedPassword string, password string) bool
	HashPassword(ctx context.Context, password string) (string, error)
	GeneratePassword() (string, error)
}

type Account interface {
	Create(ctx context.Context, ownerID, email string) (models.Account, error)
	GetByUserID(ctx context.Context, id string) ([]models.Account, error)
}

type User interface {
	Create(ctx context.Context, name, surname, email, passwordHash string) (models.User, error)
	IssueAdmin(ctx context.Context, userID, accountID string) error
	GetByEmail(ctx context.Context, email string) (models.User, error)
}

type Email interface {
	SendRegisteredMail(ctx context.Context, email, password string) error
}

//go:generate mockgen -source=./service.go -destination=./mocks/service.go -package=mock_service
type Service struct {
	Auth
	Account
	User
	Email

	repo repository.Transactable
}

func NewService(cfg config.Config, localMailBox chan string, r *repository.TransactionalRepository) *Service {
	s := &Service{
		Auth:    NewAuthService(cfg.Auth),
		Account: NewAccountService(r),
		User:    NewUserService(r),
		Email:   NewEmailService(cfg.Email, cfg.Server.Mode, localMailBox),
		repo:    r,
	}

	return s
}
