package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GroupRoleRepository struct {
	provider *ExecutorProvider
}

func NewGroupRoleRepository(provider *ExecutorProvider) *GroupRoleRepository {
	return &GroupRoleRepository{provider: provider}
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
