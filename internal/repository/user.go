package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

type UserRepository struct {
	provider *ExecutorProvider
}

func NewUserRepository(provider *ExecutorProvider) *UserRepository {
	return &UserRepository{provider: provider}
}

func (r *UserRepository) SelectByEmail(ctx context.Context, email string) ([]domain.User, error) {
	exec := r.provider.GetExecutor(ctx)

	usersDB, err := schema.Users.Query(
		sm.Where(schema.Users.Columns.Email.EQ(psql.S(email))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, nil
	}

	if usersDB == nil {
		return nil, nil
	}

	users := make([]domain.User, len(usersDB))
	for i, user := range usersDB {
		users[i] = domain.User{}
		users[i].FromDB(user)
	}

	return users, nil
}

func (r *UserRepository) Insert(ctx context.Context, name, surname, hash, email string, roleID uuid.UUID) (domain.User, error) {
	exec := r.provider.GetExecutor(ctx)

	var user domain.User

	userDB, err := schema.Users.Insert(&schema.UserSetter{
		Name:         omit.From(name),
		Surname:      omit.From(surname),
		PasswordHash: omit.From(hash),
		Email:        omit.From(email),
		RoleID:       omit.From(roleID),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return user, err
	}

	user.FromDB(userDB)

	return user, nil
}

func (r *UserRepository) SelectByID(ctx context.Context, usersID ...uuid.UUID) ([]domain.User, error) {
	exec := r.provider.GetExecutor(ctx)

	users := make([]domain.User, len(usersID))

	for i, id := range usersID {
		users[i] = domain.User{}

		userDB, err := schema.Users.Query(
			sm.Where(schema.Users.Columns.UserID.EQ(psql.Arg(id))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, nil
		}
		if userDB == nil {
			zap.L().Warn("not found: " + id.String())
			return nil, nil
		}

		users[i].FromDB(userDB)
	}

	return users, nil
}
