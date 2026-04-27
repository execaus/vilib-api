package repository

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
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
		return nil, ErrNotFound
	}

	users := make([]domain.User, len(usersDB))
	for i, user := range usersDB {
		users[i] = domain.User{}
		users[i].FromDB(user)
	}

	return users, nil
}

func (r *UserRepository) Insert(
	ctx context.Context,
	name, surname, hash, email string,
	roleID uuid.UUID,
) (domain.User, error) {
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
			if errors.Is(pgx.ErrNoRows, err) {
				return nil, ErrNotFound
			}
			zap.L().Error(err.Error())
			return nil, nil
		}

		users[i].FromDB(userDB)
	}

	return users, nil
}

func (r *UserRepository) UpdateRole(ctx context.Context, userID, roleID uuid.UUID) (domain.User, error) {
	exec := r.provider.GetExecutor(ctx)

	userDB, err := schema.Users.Update(
		(&schema.UserSetter{RoleID: omit.From(roleID)}).UpdateMod(),
		um.Where(schema.Users.Columns.UserID.EQ(psql.Arg(userID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.User{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	var user domain.User
	user.FromDB(userDB)

	return user, nil
}

func (r *UserRepository) Deactivate(ctx context.Context, userID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.Users.Update(
		(&schema.UserSetter{DeactivatedAt: omitnull.From(time.Now())}).UpdateMod(),
		um.Where(schema.Users.Columns.UserID.EQ(psql.Arg(userID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	_, err = schema.GroupMembers.Delete(
		dm.Where(schema.GroupMembers.Columns.UserID.EQ(psql.Arg(userID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

func (r *UserRepository) Reactivate(ctx context.Context, userID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	deactivatedAt := omitnull.FromPtr[time.Time](nil)
	_, err := schema.Users.Update(
		(&schema.UserSetter{DeactivatedAt: deactivatedAt}).UpdateMod(),
		um.Where(schema.Users.Columns.UserID.EQ(psql.Arg(userID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

func (r *UserRepository) SelectByAccountID(
	ctx context.Context,
	accountID uuid.UUID,
	status UserStatus,
) ([]domain.User, error) {
	exec := r.provider.GetExecutor(ctx)

	mods := []bob.Mod[*dialect.SelectQuery]{
		sm.InnerJoin(schema.AccountRoles.Name()).OnEQ(
			schema.AccountRoles.Columns.AccountRoleID,
			schema.Users.Columns.RoleID,
		),
		sm.Where(schema.AccountRoles.Columns.AccountID.EQ(psql.Arg(accountID))),
	}

	switch status {
	case UserStatusActive:
		mods = append(mods, sm.Where(schema.Users.Columns.DeactivatedAt.IsNull()))
	case UserStatusDeactivated:
		mods = append(mods, sm.Where(schema.Users.Columns.DeactivatedAt.IsNotNull()))
	}

	usersDB, err := schema.Users.Query(mods...).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	users := make([]domain.User, len(usersDB))
	for i, db := range usersDB {
		users[i] = domain.User{}
		users[i].FromDB(db)
	}

	return users, nil
}
