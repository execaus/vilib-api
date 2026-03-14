package service

import (
	"context"
	"time"
	"vilib-api/internal/gen/schema"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
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
	exec := s.repo.GetExecutor(ctx)

	permission := defaultPermission

	permission = AddPermission(permission, accountAdminBitPosition)

	_, err := schema.AccountPermissions.Insert(&schema.AccountPermissionSetter{
		UserID:     omit.From(uuid.MustParse(userID)),
		AccountID:  omit.From(uuid.MustParse(accountID)),
		Permission: omit.From(permission),
		UpdatedAt:  omit.From(time.Now()),
	}).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
