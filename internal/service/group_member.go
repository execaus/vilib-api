package service

import (
	"context"
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

// GetByUserID выбирает все членства пользователя во всех группах без проверки прав —
// используется сборкой профиля пользователя (§2.3 дизайна эпика Э2).
func (s *GroupMemberService) GetByUserID(ctx context.Context, userID uuid.UUID) ([]domain.GroupMember, error) {
	members, err := s.repo.SelectByUserID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return members, nil
}

func (s *GroupMemberService) RemoveMember(
	ctx context.Context,
	accountID, initiatorID, groupID, targetID uuid.UUID,
) error {
	// Проверка прав по общей OR-логике: право уровня аккаунта ManageGroups либо владение/
	// ManageMembers в группе (В-18, §3.1 дизайна эпика Э2).
	if err := s.srv.Access.IsCheckGroupAction(
		ctx,
		accountID, initiatorID, groupID,
		domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Удаление участника из группы
	if err := s.repo.Delete(ctx, groupID, targetID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// ListByGroup возвращает участников группы вместе с данными пользователя и его роли в группе
// (§3.2 дизайна эпика Э2, П-3): членства → батч пользователей → роли группы аккаунта → сборка.
// Право — IsCheckGroupAction(ManageGroups, GroupPermissionManageMembers): рядовому сотруднику
// список участников не нужен, доступ имеет тот же круг, что и на управление ими.
func (s *GroupMemberService) ListByGroup(
	ctx context.Context,
	accountID, initiatorID, groupID uuid.UUID,
) ([]domain.GroupMemberDetails, error) {
	if err := s.srv.Access.IsCheckGroupAction(
		ctx,
		accountID, initiatorID, groupID,
		domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
	); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	members, err := s.repo.SelectByGroupID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	if len(members) == 0 {
		return []domain.GroupMemberDetails{}, nil
	}

	usersID := make([]uuid.UUID, len(members))
	for i, member := range members {
		usersID[i] = member.UserID
	}

	users, err := s.srv.User.GetByIDs(ctx, usersID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	userByID := make(map[uuid.UUID]domain.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}

	groupRoles, err := s.srv.GroupRole.GetByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	groupRoleByID := make(map[uuid.UUID]domain.GroupRole, len(groupRoles))
	for _, role := range groupRoles {
		groupRoleByID[role.ID] = role
	}

	details := make([]domain.GroupMemberDetails, 0, len(members))
	for _, member := range members {
		user, ok := userByID[member.UserID]
		if !ok {
			zap.L().Warn("user not found for group member")
			continue
		}

		role, ok := groupRoleByID[member.RoleID]
		if !ok {
			zap.L().Warn("group role not found for group member")
			continue
		}

		details = append(details, domain.GroupMemberDetails{
			UserID:         user.ID,
			Name:           user.Name,
			Surname:        user.Surname,
			Email:          user.Email,
			RoleID:         role.ID,
			RoleName:       role.Name,
			PermissionMask: role.PermissionMask,
			JoinedAt:       member.JoinedAt,
		})
	}

	return details, nil
}

// UpdateRole меняет роль участника группы (§3.3 дизайна эпика Э2, П-4). Право —
// IsCheckGroupAction(ManageGroups, GroupPermissionManageMembers); роль должна принадлежать
// accountID — иначе ErrNotFound (не раскрываем чужие роли); участник не найден (0 строк
// UpdateRole) — тоже ErrNotFound.
func (s *GroupMemberService) UpdateRole(
	ctx context.Context,
	accountID, initiatorID, groupID, userID, roleID uuid.UUID,
) (domain.GroupMemberDetails, error) {
	if err := s.srv.Access.IsCheckGroupAction(
		ctx,
		accountID, initiatorID, groupID,
		domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMemberDetails{}, err
	}

	roles, err := s.srv.GroupRole.GetByID(ctx, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMemberDetails{}, err
	}
	if roles[0].AccountID != accountID {
		return domain.GroupMemberDetails{}, ErrNotFound
	}

	member, err := s.repo.UpdateRole(ctx, groupID, userID, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMemberDetails{}, err
	}

	users, err := s.srv.User.GetByID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMemberDetails{}, err
	}
	user := users[0]

	return domain.GroupMemberDetails{
		UserID:         user.ID,
		Name:           user.Name,
		Surname:        user.Surname,
		Email:          user.Email,
		RoleID:         roles[0].ID,
		RoleName:       roles[0].Name,
		PermissionMask: roles[0].PermissionMask,
		JoinedAt:       member.JoinedAt,
	}, nil
}
