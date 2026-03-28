package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GroupMemberRepository struct {
	provider *ExecutorProvider
}

func NewGroupMemberRepository(provider *ExecutorProvider) *GroupMemberRepository {
	return &GroupMemberRepository{provider: provider}
}

func (r *GroupMemberRepository) Insert(
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
