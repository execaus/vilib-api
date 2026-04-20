package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AccountRoleService struct {
	repo repository.AccountRole
	srv  *Service
}

func NewAccountRoleService(repo repository.AccountRole, srv *Service) *AccountRoleService {
	return &AccountRoleService{repo: repo, srv: srv}
}

func (s *AccountRoleService) GetDefault(ctx context.Context, accountID uuid.UUID) (domain.AccountRole, error) {
	// Получение всех ролей аккаунта
	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	// Поиск роли по умолчанию
	defaultRole, err := s.findDefaultRole(roles)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	return defaultRole, nil
}

func (s *AccountRoleService) CreateSystemAccountOwner(
	ctx context.Context,
	accountID uuid.UUID,
) (domain.AccountRole, error) {
	// Создание битовой маски с правами владельца
	permission := domain.SetBits(domain.DefaultPermissionMask, domain.AccountPermissionOwner)

	// Создание системной роли владельца аккаунта
	if _, err := s.repo.Insert(
		ctx,
		accountID,
		domain.AccountOwnerSystemRoleName,
		nil,
		permission,
		false,
		true,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	// Получение созданной роли
	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	return roles[0], nil
}

func (s *AccountRoleService) Create(
	ctx context.Context,
	accountID, initiatorID uuid.UUID,
	name string,
	parentID *uuid.UUID,
	permission domain.PermissionMask,
	isDefault bool,
) (domain.AccountRole, error) {
	// Разрешено ли пользователю создавать роли аккаунта
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionCreateAccountRole,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, nil
	}

	// Создание роли в базе данных
	if _, err := s.repo.Insert(ctx, accountID, name, parentID, permission, isDefault, false); err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	// Получение созданной роли
	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	return roles[0], nil
}

func (s *AccountRoleService) findDefaultRole(roles []domain.AccountRole) (domain.AccountRole, error) {
	defaultRoles := make([]domain.AccountRole, 0, len(roles))
	for _, role := range roles {
		if role.IsDefault {
			defaultRoles = append(defaultRoles, role)
		}
	}

	if len(defaultRoles) == 0 {
		return domain.AccountRole{}, ErrDefaultRoleNotFound
	}

	if len(defaultRoles) > 1 {
		return domain.AccountRole{}, ErrDefaultRolesMany
	}

	return defaultRoles[0], nil
}

func (s *AccountRoleService) GetByID(ctx context.Context, rolesID ...uuid.UUID) ([]domain.AccountRole, error) {
	// Получение ролей по ID
	roles, err := s.repo.SelectByID(ctx, rolesID...)
	if err != nil {
		zap.L().Error(err.Error())
	}

	return roles, nil
}
