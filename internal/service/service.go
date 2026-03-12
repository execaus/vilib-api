package service

import (
	"context"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
)

type Auth interface {
	GenerateToken(ctx context.Context, accountID uuid.UUID) (string, error)
	GetClaimsFromToken(ctx context.Context, token string) (*models.AuthClaims, error)
	ComparePassword(ctx context.Context, hashedPassword string, password string) bool
	HashPassword(ctx context.Context, password string) (string, error)
	GeneratePassword() (string, error)
}

//go:generate mockgen -source=./service.go -destination=./mocks/service.go -package=mocks
type Service struct {
	Auth

	repo repository.Transactable
}

func NewService(r *repository.TransactionalRepository) *Service {
	s := &Service{
		repo: r,
	}

	return s
}
