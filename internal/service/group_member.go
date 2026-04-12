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
	members, err := s.repo.Insert(ctx, groupID, roleID, usersID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return members, nil
}
