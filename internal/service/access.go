package service

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AccessService struct {
	srv *Service
}

func NewAccessService(srv *Service) *AccessService {
	return &AccessService{srv: srv}
}

func (s *AccessService) IsCheckAccountAction(
	ctx context.Context,
	accountID, initiatorID uuid.UUID, action domain.PermissionFlag,
) error {
	// Находится ли инициатор в том же аккаунте
	err := s.srv.Account.IsHasUser(ctx, accountID, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Получение данных инициатора
	initiator, err := s.srv.User.GetByID(ctx, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Получение роли инициатора
	role, err := s.srv.AccountRole.GetByID(ctx, initiator[0].RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Проверка, является ли пользователь владельцем аккаунта
	if domain.HasBit(role[0].PermissionMask, domain.AccountPermissionOwner) {
		return nil
	}

	// Проверка наличия запрашиваемого разрешения
	if domain.HasBit(role[0].PermissionMask, action) {
		return nil
	}

	return ErrForbidden
}

// IsCheckGroupAction реализует общую OR-логику доступа к операциям над группой (§3.1 дизайна
// эпика Э2): (а) группа должна принадлежать accountID — иначе группы с точки зрения этого
// аккаунта не существует (ErrNotFound); (б) право уровня аккаунта accountAction (в т.ч.
// AccountPermissionOwner) разрешает операцию без проверки членства в группе; (в) иначе
// инициатор должен состоять в группе и иметь Owner/groupAction в маске своей групповой роли —
// отсутствие членства даёт ErrForbidden, а не 500.
func (s *AccessService) IsCheckGroupAction(
	ctx context.Context,
	accountID, initiatorID, groupID uuid.UUID,
	accountAction, groupAction domain.PermissionFlag,
) error {
	// Группа должна принадлежать переданному аккаунту.
	groups, err := s.srv.UserGroup.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		zap.L().Error(err.Error())
		return err
	}

	if groups[0].AccountID != accountID {
		return ErrNotFound
	}

	// Право уровня аккаунта (в т.ч. владелец) проходит без проверки членства в группе.
	if err = s.IsCheckAccountAction(ctx, accountID, initiatorID, accountAction); err == nil {
		return nil
	}

	// Иначе — членство инициатора в группе. Отсутствие членства (в т.ч. repository.ErrNotFound)
	// — запрещено, а не 500.
	member, err := s.srv.GroupMember.GetByUserIDAndGroupID(ctx, initiatorID, groupID)
	if err != nil {
		zap.L().Warn(err.Error())
		return ErrForbidden
	}

	// Получение роли инициатора в группе.
	roles, err := s.srv.GroupRole.GetByID(ctx, member.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if len(roles) == 0 {
		return ErrForbidden
	}

	// Проверка: владелец группы либо имеет запрашиваемое групповое разрешение.
	if domain.HasBit(roles[0].PermissionMask, domain.GroupPermissionOwner) ||
		domain.HasBit(roles[0].PermissionMask, groupAction) {
		return nil
	}

	return ErrForbidden
}

// CanManageAssignments проверяет право инициатора назначать обучение в области groupID —
// группе видео на момент создания назначения (§2 дизайна эпика Э3, решение В-3): аккаунтное
// AccountPermissionManageAssignments (в т.ч. Owner) или групповое GroupPermissionManageAssignments
// (в т.ч. Owner группы) при членстве в группе. OR-логика — как у IsCheckGroupAction.
func (s *AccessService) CanManageAssignments(ctx context.Context, accountID, initiatorID, groupID uuid.UUID) error {
	return s.IsCheckGroupAction(
		ctx, accountID, initiatorID, groupID,
		domain.AccountPermissionManageAssignments, domain.GroupPermissionManageAssignments,
	)
}

// CanWatchVideo определяет, может ли userID смотреть видео группы groupID: аккаунтное право
// VideoWatch/ManageVideo ИЛИ, при членстве в группе, групповое VideoWatch/ManageVideo/Owner
// (§0 дизайна эпика Э3 — вынесенная логика VideoService.canWatch для произвольного
// пользователя, а не только инициатора запроса). Ошибки доступа к данным (в т.ч. отсутствие
// членства) не поднимаются — при любой из них считается, что доступа нет.
func (s *AccessService) CanWatchVideo(ctx context.Context, accountID, userID, groupID uuid.UUID) bool {
	if err := s.IsCheckAccountAction(ctx, accountID, userID, domain.AccountPermissionVideoWatch); err == nil {
		return true
	}

	if err := s.IsCheckAccountAction(ctx, accountID, userID, domain.AccountPermissionManageVideo); err == nil {
		return true
	}

	if s.hasGroupPermission(ctx, groupID, userID, domain.GroupPermissionVideoWatch) {
		return true
	}

	return s.hasGroupPermission(ctx, groupID, userID, domain.GroupPermissionManageVideo)
}

// CanManageVideo проверяет право инициатора управлять видео группы groupID — переименование,
// удаление, разметка глав (§2 дизайна эпика Э4): аккаунтное или групповое ManageVideo (в т.ч.
// Owner). Вынесенная логика VideoService.canManageVideo — тот же приём, каким эпик Э3 вынес
// CanWatchVideo.
func (s *AccessService) CanManageVideo(ctx context.Context, accountID, groupID, initiatorID uuid.UUID) error {
	return s.IsCheckGroupAction(
		ctx, accountID, initiatorID, groupID,
		domain.AccountPermissionManageVideo, domain.GroupPermissionManageVideo,
	)
}

// hasGroupPermission проверяет только групповое право пользователя (Owner либо конкретное
// действие в маске его роли в группе) без учёта права уровня аккаунта — вспомогательный метод
// CanWatchVideo. Отсутствие членства или ошибка чтения роли — false, а не паника/500.
func (s *AccessService) hasGroupPermission(
	ctx context.Context,
	groupID, userID uuid.UUID, action domain.PermissionFlag,
) bool {
	member, err := s.srv.GroupMember.GetByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		return false
	}

	roles, err := s.srv.GroupRole.GetByID(ctx, member.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return false
	}
	if len(roles) == 0 {
		return false
	}

	return domain.HasBit(roles[0].PermissionMask, domain.GroupPermissionOwner) ||
		domain.HasBit(roles[0].PermissionMask, action)
}

// ManagedAssignmentGroups определяет область чтения отчётов по назначениям инициатора (В-8
// решение владельца): all=true — аккаунтное право ManageAssignments или Owner (видны все
// назначения аккаунта); иначе — список групп, где у инициатора роль с битом Owner или
// GroupPermissionManageAssignments (видны назначения только этих групп плюс собственные,
// проверяется вызывающим сервисом по created_by).
func (s *AccessService) ManagedAssignmentGroups(
	ctx context.Context,
	accountID, initiatorID uuid.UUID,
) (bool, []uuid.UUID, error) {
	err := s.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageAssignments)
	if err == nil {
		return true, nil, nil
	}

	members, err := s.srv.GroupMember.GetByUserID(ctx, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return false, nil, err
	}
	if len(members) == 0 {
		return false, nil, nil
	}

	groupRoles, err := s.srv.GroupRole.GetByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return false, nil, err
	}
	groupRoleByID := make(map[uuid.UUID]domain.GroupRole, len(groupRoles))
	for _, role := range groupRoles {
		groupRoleByID[role.ID] = role
	}

	groups := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		role, ok := groupRoleByID[member.RoleID]
		if !ok {
			continue
		}

		if domain.HasBit(role.PermissionMask, domain.GroupPermissionOwner) ||
			domain.HasBit(role.PermissionMask, domain.GroupPermissionManageAssignments) {
			groups = append(groups, member.GroupID)
		}
	}

	return false, groups, nil
}
