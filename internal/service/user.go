package service

import (
	"context"
	"vilib-api/internal/gen/schema"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"

	"github.com/aarondl/opt/omit"
	"go.uber.org/zap"
)

type UserService struct {
	repo *repository.TransactionalRepository
}

func NewUserService(repo *repository.TransactionalRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, name, surname, email, passwordHash string) (models.User, error) {
	exec := s.repo.GetExecutor(ctx)

	var user models.User

	userDB, err := schema.Users.Insert(&schema.UserSetter{
		Name:         omit.From(name),
		Surname:      omit.From(surname),
		PasswordHash: omit.From(passwordHash),
		Email:        omit.From(email),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return user, err
	}

	user.FromDB(userDB)

	return user, nil
}

func (s *UserService) IssueAdmin(ctx context.Context, userID, accountID string) error {
	//TODO implement me
	panic("implement me")
}
