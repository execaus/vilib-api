package service

import (
	"context"
	"time"
	"vilib-api/internal/gen/schema"
	"vilib-api/internal/models"
	"vilib-api/internal/repository"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
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

func (s *UserService) GetByEmail(ctx context.Context, email string) (models.User, error) {
	exec := s.repo.GetExecutor(ctx)

	dbUser, err := schema.Users.Query(sm.Where(schema.Users.Columns.UserID.EQ(psql.S(email)))).One(ctx, exec)
	if err != nil {
		if dbUser == nil {
			return models.User{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return models.User{}, err
	}

	user := models.User{}
	user.FromDB(dbUser)

	return user, nil
}
