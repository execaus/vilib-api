package service

import (
	"context"
	"fmt"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserGroupService struct {
	repo repository.UserGroup
	srv  *Service
}

func NewUserGroupService(repo repository.UserGroup, srv *Service) *UserGroupService {
	return &UserGroupService{repo: repo, srv: srv}
}

func (s *UserGroupService) Create(
	ctx context.Context,
	accountID, initiatorID uuid.UUID,
	name string,
) (domain.UserGroup, error) {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionCreateUserGroup,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

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
	if err := s.isAccessAddMembers(ctx, accountID, initiatorID, groupID, targetsID...); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	defaultRole, err := s.srv.GetDefault(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	members, err := s.srv.GroupMember.Create(ctx, groupID, defaultRole.ID, targetsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return members, nil
}

func (s *UserGroupService) isAccessAddMembers(
	ctx context.Context,
	accountID, initiatorID, groupID uuid.UUID,
	targetsID ...uuid.UUID,
) error {
	// Находится ли инициатор в той же организации
	err := s.srv.Account.IsHasUser(ctx, accountID, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Находится ли группа в переданном аккаунте
	group, err := s.repo.GetByID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if group[0].AccountID != accountID {
		return fmt.Errorf("%w: group does not belong to the specified account", ErrForbidden)
	}

	// Все ли пользователи находятся в аккаунте с группой
	users, err := s.srv.User.GetByID(ctx, targetsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	userRolesID := make([]uuid.UUID, len(users))
	for i, user := range users {
		userRolesID[i] = user.RoleID
	}

	roles, err := s.srv.AccountRole.GetByID(ctx, userRolesID...)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, accountRole := range roles {
		if accountRole.AccountID != accountID {
			return fmt.Errorf("%w: one or more users do not belong to the specified account", ErrForbidden)
		}
	}

	// Имеет ли роль владельца организации
	initiator, err := s.srv.User.GetByID(ctx, initiatorID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	role, err := s.srv.AccountRole.GetByID(ctx, initiator[0].RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if domain.HasBit(role[0].PermissionMask, domain.AccountPermissionOwner) {
		return nil
	}

	// Имеет ли роль разрешение на добавление пользователей в группы в организации
	if domain.HasBit(role[0].PermissionMask, domain.AccountPermissionUserGroupAddMember) {
		return nil
	}

	return nil
}
