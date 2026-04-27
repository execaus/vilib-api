package repository

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

type AccountRoleRepository struct {
	provider *ExecutorProvider
}

func (r *AccountRoleRepository) SelectByID(ctx context.Context, rolesID ...uuid.UUID) ([]domain.AccountRole, error) {
	exec := r.provider.GetExecutor(ctx)

	roles := make([]domain.AccountRole, len(rolesID))
	for i, roleID := range rolesID {
		accountRolesDB, err := schema.AccountRoles.Query(
			sm.Where(schema.AccountRoles.Columns.AccountRoleID.EQ(psql.Arg(roleID))),
		).One(ctx, exec)
		if err != nil {
			if errors.Is(pgx.ErrNoRows, err) {
				return nil, ErrNotFound
			}
			zap.L().Error(err.Error())
			return nil, err
		}

		roles[i] = domain.AccountRole{}
		roles[i].FromDB(accountRolesDB)
	}

	return roles, nil
}

func NewAccountRoleRepository(provider *ExecutorProvider) *AccountRoleRepository {
	return &AccountRoleRepository{provider: provider}
}

func (r *AccountRoleRepository) Insert(
	ctx context.Context,
	accountID uuid.UUID,
	name string,
	parentID *uuid.UUID,
	permission domain.PermissionMask,
	isDefault, isSystem bool,
) (domain.AccountRole, error) {
	exec := r.provider.GetExecutor(ctx)

	roleDB, err := schema.AccountRoles.Insert(&schema.AccountRoleSetter{
		Name:           omit.From(name),
		PermissionMask: omit.From(permission),
		AccountID:      omit.From(accountID),
		ParentRoleID:   omitnull.FromPtr(parentID),
		IsDefault:      omit.From(isDefault),
		IsSystem:       omit.From(isSystem),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	role := domain.AccountRole{}
	role.FromDB(roleDB)

	return role, nil
}

func (r *AccountRoleRepository) SelectByAccountID(
	ctx context.Context,
	accountID uuid.UUID,
) ([]domain.AccountRole, error) {
	exec := r.provider.GetExecutor(ctx)

	accountRolesDB, err := schema.AccountRoles.Query(
		sm.Where(schema.AccountRoles.Columns.AccountID.EQ(psql.Arg(accountID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	if accountRolesDB == nil {
		return nil, ErrNotFound
	}

	accountRoles := make([]domain.AccountRole, len(accountRolesDB))
	for i, role := range accountRolesDB {
		accountRoles[i] = domain.AccountRole{}
		accountRoles[i].FromDB(role)
	}

	return accountRoles, nil
}

func (r *AccountRoleRepository) Delete(ctx context.Context, roleID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.AccountRoles.Delete(
		dm.Where(schema.AccountRoles.Columns.AccountRoleID.EQ(psql.Arg(roleID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

func (r *AccountRoleRepository) SelectActiveUsersByRole(
	ctx context.Context,
	roleID uuid.UUID,
) ([]domain.User, error) {
	exec := r.provider.GetExecutor(ctx)

	usersDB, err := schema.Users.Query(
		sm.Where(schema.Users.Columns.RoleID.EQ(psql.Arg(roleID))),
		sm.Where(schema.Users.Columns.DeactivatedAt.IsNull()),
	).All(ctx, exec)
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

func (r *AccountRoleRepository) ResetRoleToDefault(
	ctx context.Context,
	oldRoleID, defaultRoleID uuid.UUID,
) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.Users.Update(
		(&schema.UserSetter{RoleID: omit.From(defaultRoleID)}).UpdateMod(),
		um.Where(schema.Users.Columns.RoleID.EQ(psql.Arg(oldRoleID))),
		um.Where(schema.Users.Columns.DeactivatedAt.IsNotNull()),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
