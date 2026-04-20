package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GroupRoleService struct {
	srv  *Service
	repo repository.GroupRole
}

func NewGroupRoleService(repo repository.GroupRole, srv *Service) *GroupRoleService {
	return &GroupRoleService{repo: repo, srv: srv}
}

func (s *GroupRoleService) Create(
	ctx context.Context,
	accountID, initiatorID uuid.UUID,
	name string,
	permission domain.PermissionMask,
	isDefault bool,
) (domain.GroupRole, error) {
	// Проверка прав доступа на создание роли группы
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionCreateAccountRole,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	// Создание роли группы
	role, err := s.repo.Insert(ctx, accountID, name, permission, isDefault)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, nil
	}

	return role, nil
}

func (s *GroupRoleService) GetByID(ctx context.Context, roleID uuid.UUID) ([]domain.GroupRole, error) {
	// Получение роли группы по ID
	roles, err := s.repo.SelectByID(ctx, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return roles, nil
}

func (s *GroupRoleService) GetDefault(ctx context.Context, accountID uuid.UUID) (domain.GroupRole, error) {
	// Получение дефолтной роли группы для аккаунта
	role, err := s.repo.GetDefault(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	return role, nil
}
