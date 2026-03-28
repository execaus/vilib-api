package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserGroupService struct {
	repo repository.UserGroup
	srv  *Service
}

func (s *UserGroupService) Create(
	ctx context.Context,
	accountID, initiatorID uuid.UUID,
	name string,
) (domain.UserGroup, error) {
	group, err := s.repo.Insert(ctx, accountID, name)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, nil
	}

	return group, nil
}

func (s *UserGroupService) AddMembers(
	ctx context.Context,
	accountID, initiatorID, groupID uuid.UUID,
	targetsID ...uuid.UUID,
) ([]domain.GroupMember, error) {
	// TODO get default role id in account

	//roleID := uuid.New()
	//
	//members, err := s.repo.InsertMembers(ctx, groupID, roleID, targetsID...)
	//if err != nil {
	//	zap.L().Error(err.Error())
	//	return nil, err
	//}

	return nil, nil
}

func NewUserGroupService(repo repository.UserGroup, srv *Service) *UserGroupService {
	return &UserGroupService{repo: repo, srv: srv}
}
