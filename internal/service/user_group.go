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
	// Проверка прав доступа на создание группы (только владельцы аккаунта могут создавать группы)
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageUsers,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	// Создание группы пользователей
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
	// Проверка прав доступа на добавление участников
	if err := s.isAccessAddMembers(ctx, accountID, initiatorID, groupID, targetsID...); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Получение дефолтной роли группы для аккаунта
	defaultRole, err := s.srv.GroupRole.GetDefault(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Добавление участников в группу
	members, err := s.srv.GroupMember.Create(ctx, groupID, defaultRole.ID, targetsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return members, nil
}

func (s *UserGroupService) GetAll(
	ctx context.Context,
	initiatorID, accountID uuid.UUID,
) ([]domain.UserGroup, error) {
	// Проверка, что инициатор является участником аккаунта
	if err := s.srv.Account.IsHasUser(ctx, accountID, initiatorID); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Получение всех групп аккаунта
	groups, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return groups, nil
}

func (s *UserGroupService) Delete(
	ctx context.Context,
	initiatorID, accountID, groupID uuid.UUID,
) error {
	// Проверка прав на управление группами
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageGroups); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Удаление группы каскадно
	if err := s.repo.DeleteCascade(ctx, groupID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
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

	// Получение роли инициатора в группе
	initiatorGroupMember, err := s.srv.GroupMember.GetByUserIDAndGroupID(ctx, initiatorID, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Получение group role инициатора
	groupRoles, err := s.srv.GroupRole.GetByID(ctx, initiatorGroupMember.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Проверка: является ли владельцем группы
	if domain.HasBit(groupRoles[0].PermissionMask, domain.GroupPermissionOwner) {
		return nil
	}

	// Проверка: имеет ли право на добавление участников
	if domain.HasBit(groupRoles[0].PermissionMask, domain.GroupPermissionManageMembers) {
		return nil
	}

	return ErrForbidden
}
