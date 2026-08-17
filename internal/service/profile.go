package service

import (
	"context"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ProfileService агрегирует контекст текущего пользователя для ручки GET /me (§2.3 дизайна
// эпика Э2). Не владеет собственной таблицей — собирает ответ из других сервисов.
type ProfileService struct {
	srv *Service
}

// NewProfileService создаёт ProfileService.
func NewProfileService(srv *Service) *ProfileService {
	return &ProfileService{srv: srv}
}

// Get собирает профиль пользователя userID (§2.3 дизайна эпика Э2): организацию текущей
// строки, все организации по email с активной строкой, роль в текущей организации, признак
// владельца аккаунта и членства в группах. Право доступа — любой аутентифицированный
// пользователь; деактивированная строка запрещает вход в профиль.
func (s *ProfileService) Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error) {
	users, err := s.srv.User.GetByID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Profile{}, err
	}
	user := users[0]

	// Деактивированная строка не может смотреть свой профиль — фронт разлогинивает.
	if !user.IsActive() {
		return domain.Profile{}, ErrForbiddenUserDeactivated
	}

	roles, err := s.srv.AccountRole.GetByID(ctx, user.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Profile{}, err
	}
	role := roles[0]

	accounts, err := s.srv.Account.GetByID(ctx, role.AccountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Profile{}, err
	}
	account := accounts[0]

	// Все организации по email с активной строкой (§2.4 дизайна эпика, A-03 ТЗ).
	allAccounts, err := s.srv.Account.GetByUserEmail(ctx, user.Email)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Profile{}, err
	}

	groups, err := s.groups(ctx, userID, account.ID)
	if err != nil {
		return domain.Profile{}, err
	}

	return domain.Profile{
		User:     user,
		Account:  account,
		Accounts: allAccounts,
		Role:     role,
		IsOwner:  domain.HasBit(role.PermissionMask, domain.AccountPermissionOwner),
		Groups:   groups,
	}, nil
}

// groups собирает членства пользователя в группах аккаунта: сначала членства, затем батчем
// названия групп и роли групп аккаунта (без N+1 на каждое членство).
func (s *ProfileService) groups(ctx context.Context, userID, accountID uuid.UUID) ([]domain.GroupMembership, error) {
	members, err := s.srv.GroupMember.GetByUserID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	groups := make([]domain.GroupMembership, 0, len(members))
	if len(members) == 0 {
		return groups, nil
	}

	groupsID := make([]uuid.UUID, len(members))
	for i, member := range members {
		groupsID[i] = member.GroupID
	}

	userGroups, err := s.srv.UserGroup.GetByID(ctx, groupsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}
	groupNameByID := make(map[uuid.UUID]string, len(userGroups))
	for _, group := range userGroups {
		groupNameByID[group.ID] = group.Name
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

	for _, member := range members {
		groupRole, ok := groupRoleByID[member.RoleID]
		if !ok {
			zap.L().Warn("group role not found for group member")
			continue
		}

		groups = append(groups, domain.GroupMembership{
			GroupID:        member.GroupID,
			GroupName:      groupNameByID[member.GroupID],
			RoleID:         groupRole.ID,
			RoleName:       groupRole.Name,
			PermissionMask: groupRole.PermissionMask,
		})
	}

	return groups, nil
}
