package service

import (
	"context"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"
)

type Auth interface {
	GenerateToken(ctx context.Context, userID, accountID string) (string, error)
	GetClaimsFromToken(ctx context.Context, token string) (*models.AuthClaims, error)
	ComparePassword(ctx context.Context, hashedPassword string, password string) bool
	HashPassword(ctx context.Context, password string) (string, error)
	GeneratePassword() (string, error)
}

type Account interface {
	Create(ctx context.Context, ownerID, email string) (models.Account, error)
}

type User interface {
	Create(ctx context.Context, name, surname, email, passwordHash string) (models.User, error)
	IssueAdmin(ctx context.Context, userID, accountID string) error
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

func NewService(r *repository.TransactionalRepository) *Service {
	s := &Service{
		repo: r,
	}

	return s
}
