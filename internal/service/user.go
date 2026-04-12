package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserService struct {
	repo repository.User
	srv  *Service
}

func NewUserService(repo repository.User, srv *Service) *UserService {
	return &UserService{repo: repo, srv: srv}
}

func (s *UserService) Create(
	ctx context.Context,
	name, surname, email, passwordHash string,
	roleID uuid.UUID,
) (domain.User, error) {
	user, err := s.repo.Insert(ctx, name, surname, passwordHash, email, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return user, err
	}

	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) ([]domain.User, error) {
	users, err := s.repo.SelectByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}

func (s *UserService) Update(
	ctx context.Context,
	initiatorID, targetUserID uuid.UUID,
	roleID *uuid.UUID,
) (domain.User, error) {

	return domain.User{}, nil
}

func (s *UserService) GetByID(ctx context.Context, userID ...uuid.UUID) ([]domain.User, error) {
	users, err := s.repo.SelectByID(ctx, userID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}
