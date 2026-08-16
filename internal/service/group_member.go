package service

import (
	"context"
	"fmt"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GroupMemberService struct {
	srv  *Service
	repo repository.GroupMember
}

func NewGroupMemberService(repo repository.GroupMember, srv *Service) *GroupMemberService {
	return &GroupMemberService{repo: repo, srv: srv}
}

func (s *GroupMemberService) Create(
	ctx context.Context,
	groupID, roleID uuid.UUID,
	usersID ...uuid.UUID,
) ([]domain.GroupMember, error) {
	// Добавление пользователей в группу
	members, err := s.repo.Insert(ctx, groupID, roleID, usersID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return members, nil
}

func (s *GroupMemberService) GetByUserIDAndGroupID(
	ctx context.Context,
	userID, groupID uuid.UUID,
) (domain.GroupMember, error) {
	// Получение участника группы по userID и groupID
	member, err := s.repo.SelectByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMember{}, err
	}

	return member, nil
}

func (s *GroupMemberService) RemoveMember(
	ctx context.Context,
	accountID, initiatorID, groupID, targetID uuid.UUID,
) error {
	if err := s.isAccessRemoveMember(ctx, accountID, initiatorID, groupID); err != nil {
		return err
	}

	// Удаление участника из группы
	if err := s.repo.Delete(ctx, groupID, targetID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// isAccessRemoveMember реализует OR-логику проверки прав на удаление участника (В-18, §6.4
// ТЗ): право уровня аккаунта ManageGroups (или владелец аккаунта) достаточно — членство
// инициатора в самой группе не требуется, но группа должна принадлежать переданному аккаунту;
// иначе действует групповая проверка (Owner/ManageMembers в самой группе).
func (s *GroupMemberService) isAccessRemoveMember(
	ctx context.Context,
	accountID, initiatorID, groupID uuid.UUID,
) error {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx, accountID, initiatorID, domain.AccountPermissionManageGroups,
	); err == nil {
		return s.isGroupInAccount(ctx, initiatorID, accountID, groupID)
	}

	return s.isGroupMemberAllowedToRemove(ctx, initiatorID, groupID)
}

// isGroupInAccount проверяет, что группа принадлежит переданному аккаунту — исключает
// использование accountID от другого аккаунта для обхода групповой проверки.
func (s *GroupMemberService) isGroupInAccount(
	ctx context.Context,
	initiatorID, accountID, groupID uuid.UUID,
) error {
	groups, err := s.srv.UserGroup.GetAll(ctx, initiatorID, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, group := range groups {
		if group.ID == groupID {
			return nil
		}
	}

	return fmt.Errorf("%w: group does not belong to the specified account", ErrForbidden)
}

// isGroupMemberAllowedToRemove проверяет, состоит ли инициатор в группе и имеет ли право
// Owner/ManageMembers. Если инициатор не состоит в группе (в т.ч. repository.ErrNotFound/
// "sql: no rows") — запрещено, а не 500.
func (s *GroupMemberService) isGroupMemberAllowedToRemove(
	ctx context.Context,
	initiatorID, groupID uuid.UUID,
) error {
	member, err := s.repo.SelectByUserIDAndGroupID(ctx, initiatorID, groupID)
	if err != nil {
		zap.L().Warn(err.Error())
		return ErrForbidden
	}

	roles, err := s.srv.GroupRole.GetByID(ctx, member.RoleID)
	if err != nil || len(roles) == 0 {
		return ErrForbidden
	}

	if domain.HasBit(roles[0].PermissionMask, domain.GroupPermissionOwner) ||
		domain.HasBit(roles[0].PermissionMask, domain.GroupPermissionManageMembers) {
		return nil
	}

	return ErrForbidden
}
