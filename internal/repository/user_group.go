package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserGroupRepository struct {
	provider *ExecutorProvider
}

func NewUserGroupRepository(provider *ExecutorProvider) *UserGroupRepository {
	return &UserGroupRepository{provider: provider}
}

func (r *UserGroupRepository) Insert(
	ctx context.Context,
	accountID uuid.UUID,
	name string,
) (domain.UserGroup, error) {
	exec := r.provider.GetExecutor(ctx)

	userGroupDB, err := schema.UserGroups.Insert(&schema.UserGroupSetter{
		Name:      omit.From(name),
		AccountID: omit.From(accountID),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, nil
	}

	userGroup := domain.UserGroup{}
	userGroup.FromDB(userGroupDB)

	return userGroup, nil
}
