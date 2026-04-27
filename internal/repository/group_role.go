package repository

import (
	"context"
	"fmt"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

type GroupRoleRepository struct {
	provider *ExecutorProvider
}

func NewGroupRoleRepository(provider *ExecutorProvider) *GroupRoleRepository {
	return &GroupRoleRepository{provider: provider}
}

func (r *GroupRoleRepository) SelectByID(ctx context.Context, roleID uuid.UUID) ([]domain.GroupRole, error) {
	exec := r.provider.GetExecutor(ctx)

	rolesDB, err := schema.GroupRoles.Query(
		sm.Where(schema.GroupRoles.Columns.GroupRoleID.EQ(psql.Arg(roleID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	if rolesDB == nil {
		return nil, ErrNotFound
	}

	roles := make([]domain.GroupRole, len(rolesDB))
	for i, role := range rolesDB {
		roles[i] = domain.GroupRole{}
		roles[i].FromDB(role)
	}

	return roles, nil
}

func (r *GroupRoleRepository) SelectByAccount(ctx context.Context, accountID uuid.UUID) ([]domain.GroupRole, error) {
	exec := r.provider.GetExecutor(ctx)

	rolesDB, err := schema.GroupRoles.Query(
		sm.Where(schema.GroupRoles.Columns.AccountID.EQ(psql.Arg(accountID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	if rolesDB == nil {
		return nil, ErrNotFound
	}

	roles := make([]domain.GroupRole, len(rolesDB))
	for i, role := range rolesDB {
		roles[i] = domain.GroupRole{}
		roles[i].FromDB(role)
	}

	return roles, nil
}

func (r *GroupRoleRepository) Insert(
	ctx context.Context,
	accountID uuid.UUID,
	name string,
	permission domain.PermissionMask,
	isDefault bool,
) (domain.GroupRole, error) {
	exec := r.provider.GetExecutor(ctx)

	roleDB, err := schema.GroupRoles.Insert(&schema.GroupRoleSetter{
		GroupRoleID:    omit.Val[uuid.UUID]{},
		Name:           omit.From(name),
		PermissionMask: omit.From(permission),
		AccountID:      omit.From(accountID),
		IsDefault:      omit.From(isDefault),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	role := domain.GroupRole{}
	role.FromDB(roleDB)

	return role, nil
}

func (r *GroupRoleRepository) GetDefault(ctx context.Context, accountID uuid.UUID) (domain.GroupRole, error) {
	exec := r.provider.GetExecutor(ctx)

	rolesDB, err := schema.GroupRoles.Query(
		sm.Where(schema.GroupRoles.Columns.AccountID.EQ(psql.Arg(accountID))),
		sm.Where(schema.GroupRoles.Columns.IsDefault.EQ(psql.Arg(true))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	if len(rolesDB) == 0 {
		return domain.GroupRole{}, ErrNotFound
	}

	if len(rolesDB) > 1 {
		return domain.GroupRole{}, fmt.Errorf("%w: multiple default roles found", ErrNotFound)
	}

	role := domain.GroupRole{}
	role.FromDB(rolesDB[0])

	return role, nil
}

func (r *GroupRoleRepository) SelectMembersByRole(
	ctx context.Context,
	roleID uuid.UUID,
) ([]domain.GroupMember, error) {
	exec := r.provider.GetExecutor(ctx)

	membersDB, err := schema.GroupMembers.Query(
		sm.Where(schema.GroupMembers.Columns.RoleID.EQ(psql.Arg(roleID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	members := make([]domain.GroupMember, len(membersDB))
	for i, m := range membersDB {
		members[i] = domain.GroupMember{}
		members[i].FromDB(m)
	}

	return members, nil
}

func (r *GroupRoleRepository) Delete(ctx context.Context, roleID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.GroupRoles.Delete(
		dm.Where(schema.GroupRoles.Columns.GroupRoleID.EQ(psql.Arg(roleID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
