package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
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

func (r *GroupMemberRepository) SelectByUserIDAndGroupID(
	ctx context.Context,
	userID, groupID uuid.UUID,
) (domain.GroupMember, error) {
	exec := r.provider.GetExecutor(ctx)

	member, err := schema.GroupMembers.Query(
		sm.Where(schema.GroupMembers.Columns.UserID.EQ(psql.Arg(userID))),
		sm.Where(schema.GroupMembers.Columns.GroupID.EQ(psql.Arg(groupID))),
	).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMember{}, err
	}

	var dm domain.GroupMember
	dm.FromDB(member)
	return dm, nil
}

func (r *GroupMemberRepository) Delete(ctx context.Context, groupID, userID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	rowsAffected, err := schema.GroupMembers.Delete(
		dm.Where(schema.GroupMembers.Columns.GroupID.EQ(psql.Arg(groupID))),
		dm.Where(schema.GroupMembers.Columns.UserID.EQ(psql.Arg(userID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
