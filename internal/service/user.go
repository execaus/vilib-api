package service

import (
	"context"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"
	"vilib-api/internal/repository"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

type UserService struct {
	repo *repository.TransactionalRepository
}

func NewUserService(repo *repository.TransactionalRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, name, surname, email, passwordHash string) (domain.User, error) {
	exec := s.repo.GetExecutor(ctx)

	var user domain.User

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
	return s.manipulationPermission(ctx, userID, accountID, func(perm BitmapPermission) BitmapPermission {
		return AddPermission(perm, accountAdminBitPosition)
	})
}

func (s *UserService) IssueUser(ctx context.Context, userID, accountID string) error {
	return s.manipulationPermission(ctx, userID, accountID, func(perm BitmapPermission) BitmapPermission {
		return AddPermission(perm, accountUserBitPosition)
	})
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	exec := s.repo.GetExecutor(ctx)

	dbUser, err := schema.Users.Query(sm.Where(schema.Users.Columns.Email.EQ(psql.S(email)))).One(ctx, exec)
	if err != nil {
		if dbUser == nil {
			return domain.User{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	user := domain.User{}
	user.FromDB(dbUser)

	return user, nil
}

func (s *UserService) getCurrentPermission(
	ctx context.Context,
	userID, accountID string,
) (*schema.AccountPermission, error) {
	exec := s.repo.GetExecutor(ctx)

	permissionDB, err := schema.AccountPermissions.Query(
		sm.Where(schema.AccountPermissions.Columns.UserID.EQ(psql.S(userID))),
		sm.Where(schema.AccountPermissions.Columns.AccountID.EQ(psql.S(accountID))),
	).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return permissionDB, nil
}

func (s *UserService) applyPermission(
	ctx context.Context,
	userID, accountID string,
	permissionDB *schema.AccountPermission,
	permission BitmapPermission,
) error {
	exec := s.repo.GetExecutor(ctx)

	if permissionDB == nil {
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
	} else {
		_, err := schema.AccountPermissions.Update(
			um.SetCol(schema.AccountPermissions.Columns.Permission.String()).ToArg(permission),
			um.SetCol(schema.AccountPermissions.Columns.UpdatedAt.String()).ToArg(time.Now()),
			um.Where(schema.AccountPermissions.Columns.UserID.EQ(psql.Arg(userID))),
			um.Where(schema.AccountPermissions.Columns.AccountID.EQ(psql.Arg(accountID))),
		).Exec(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}
	}

	return nil
}

func (s *UserService) manipulationPermission(
	ctx context.Context,
	userID, accountID string,
	fn func(perm BitmapPermission) BitmapPermission,
) error {
	permissionDB, err := s.getCurrentPermission(ctx, userID, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	permission := fn(extractPermissionValue(permissionDB))

	if err = s.applyPermission(ctx, userID, accountID, permissionDB, permission); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

func extractPermissionValue(row *schema.AccountPermission) BitmapPermission {
	if row == nil {
		return defaultPermission
	}

	return row.Permission
}
