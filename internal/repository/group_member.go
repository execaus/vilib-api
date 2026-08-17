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
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
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

// SelectByUserID выбирает все членства пользователя во всех группах (для агрегации профиля
// GET /me, §2.3 дизайна эпика). Отсутствие членств — не ошибка, возвращается пустой срез.
func (r *GroupMemberRepository) SelectByUserID(ctx context.Context, userID uuid.UUID) ([]domain.GroupMember, error) {
	exec := r.provider.GetExecutor(ctx)

	membersDB, err := schema.GroupMembers.Query(
		sm.Where(schema.GroupMembers.Columns.UserID.EQ(psql.Arg(userID))),
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

// SelectByGroupID выбирает всех участников группы — список участников карточки группы
// (§3.2 дизайна эпика Э2, П-3). Отсутствие участников — не ошибка, возвращается пустой срез.
func (r *GroupMemberRepository) SelectByGroupID(ctx context.Context, groupID uuid.UUID) ([]domain.GroupMember, error) {
	exec := r.provider.GetExecutor(ctx)

	membersDB, err := schema.GroupMembers.Query(
		sm.Where(schema.GroupMembers.Columns.GroupID.EQ(psql.Arg(groupID))),
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

// UpdateRole меняет роль участника группы (§3.3 дизайна эпика Э2, П-4). Участник не найден
// (0 строк) — ErrNotFound.
func (r *GroupMemberRepository) UpdateRole(
	ctx context.Context,
	groupID, userID, roleID uuid.UUID,
) (domain.GroupMember, error) {
	exec := r.provider.GetExecutor(ctx)

	memberDB, err := schema.GroupMembers.Update(
		(&schema.GroupMemberSetter{RoleID: omit.From(roleID)}).UpdateMod(),
		um.Where(schema.GroupMembers.Columns.GroupID.EQ(psql.Arg(groupID))),
		um.Where(schema.GroupMembers.Columns.UserID.EQ(psql.Arg(userID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.GroupMember{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.GroupMember{}, err
	}

	var member domain.GroupMember
	member.FromDB(memberDB)

	return member, nil
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
