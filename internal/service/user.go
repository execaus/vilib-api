package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
)

type UserService struct {
	repo *repository.Repository
	srv  *Service
}

func NewUserService(repo *repository.Repository, srv *Service) *UserService {
	return &UserService{repo: repo, srv: srv}
}

func (s *UserService) Create(ctx context.Context, name, surname, email, passwordHash string) (domain.User, error) {
	user, err := s.repo.User.Insert(ctx, name, surname, passwordHash, email)
	if err != nil {
		zap.L().Error(err.Error())
		return user, err
	}

	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) ([]domain.User, error) {
	users, err := s.repo.User.SelectByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}
