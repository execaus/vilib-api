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
