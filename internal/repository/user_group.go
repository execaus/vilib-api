package repository

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
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
		return domain.UserGroup{}, err
	}

	userGroup := domain.UserGroup{}
	userGroup.FromDB(userGroupDB)

	return userGroup, nil
}

func (r *UserGroupRepository) GetByID(ctx context.Context, groupsID ...uuid.UUID) ([]domain.UserGroup, error) {
	exec := r.provider.GetExecutor(ctx)

	userGroups := make([]domain.UserGroup, len(groupsID))

	for i, id := range groupsID {
		userGroupDB, err := schema.UserGroups.Query(
			sm.Where(schema.UserGroups.Columns.GroupID.EQ(psql.Arg(id))),
		).One(ctx, exec)
		if err != nil {
			if errors.Is(pgx.ErrNoRows, err) {
				return nil, ErrNotFound
			}
			zap.L().Error(err.Error())
			return nil, err
		}

		userGroups[i] = domain.UserGroup{}
		userGroups[i].FromDB(userGroupDB)
	}

	return userGroups, nil
}
