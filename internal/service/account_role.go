package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
)

type AccountRoleService struct {
	repo repository.AccountRole
	srv  *Service
}

func NewAccountRoleService(repo repository.AccountRole, srv *Service) *AccountRoleService {
	return &AccountRoleService{repo: repo, srv: srv}
}

func (s *AccountRoleService) Create(
	ctx context.Context,
	accountID, name string, parentID *string,
	permission domain.PermissionMask,
	isDefault bool,
) ([]domain.AccountRole, error) {
	if _, err := s.repo.Insert(ctx, accountID, name, parentID, permission, isDefault); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return roles, nil
}
