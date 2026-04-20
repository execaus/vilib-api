package service

import (
	"context"
	"vilib-api/internal/domain"

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
