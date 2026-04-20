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
	// Добавление пользователей в группу
	members, err := s.repo.Insert(ctx, groupID, roleID, usersID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return members, nil
}

func (s *GroupMemberService) GetByUserIDAndGroupID(
	ctx context.Context,
	userID, groupID uuid.UUID,
) (domain.GroupMember, error) {
	// Получение участника группы по userID и groupID
	member, err := s.repo.SelectByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.GroupMember{}, err
	}

	return member, nil
}
