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

func (r *UserGroupRepository) InsertGroup(
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

func (r *UserGroupRepository) InsertMembers(
	ctx context.Context,
	groupID, roleID uuid.UUID,
	usersID ...uuid.UUID,
) ([]domain.GroupMember, error) {
	exec := r.provider.GetExecutor(ctx)

	members := make([]domain.GroupMember, len(usersID))
	for i, id := range usersID {
		member, err := schema.GroupMembers.Insert(&schema.GroupMemberSetter{
			UserID:  omit.From(id),
			GroupID: omit.From(groupID),
			RoleID:  omit.From(roleID),
		}).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		members[i] = domain.GroupMember{}
		members[i].FromDB(member)
	}

	return members, nil
}

func NewUserGroupRepository(provider *ExecutorProvider) *UserGroupRepository {
	return &UserGroupRepository{provider: provider}
}
