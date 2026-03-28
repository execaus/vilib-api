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
	accountID uuid.UUID,
	name string,
	permission domain.PermissionMask,
	isDefault bool,
) (domain.GroupRole, error) {
	role, err := s.repo.Insert(ctx, accountID, name, permission, isDefault)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, nil
	}

	return role, nil
}
