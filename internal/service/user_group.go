package service

import (
	"context"
	"errors"
	"fmt"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
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
	// Проверка прав доступа на создание группы (§6.4 ТЗ: право ManageGroups уровня аккаунта)
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageGroups,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	// Создание группы пользователей
	group, err := s.repo.Insert(ctx, accountID, name)
	if err != nil {
		if errors.Is(dberrors.UserGroupErrors.ErrUniqueUserGroupsNameAccountIdKey, err) {
			zap.L().Warn(err.Error())
			return domain.UserGroup{}, ErrGroupNameExists
		}
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	return group, nil
}

// Rename переименовывает группу (§4 дизайна эпика Э2, «Блок C — редактирование»). Право —
// ManageGroups; группа должна принадлежать accountID (иначе не раскрываем чужие группы —
// ErrNotFound); дубль имени в пределах аккаунта — ErrGroupNameExists.
func (s *UserGroupService) Rename(
	ctx context.Context,
	initiatorID, accountID, groupID uuid.UUID,
	name string,
) (domain.UserGroup, error) {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageGroups,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	groups, err := s.repo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.UserGroup{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}
	if groups[0].AccountID != accountID {
		return domain.UserGroup{}, ErrNotFound
	}

	group, err := s.repo.UpdateName(ctx, groupID, name)
	if err != nil {
		if errors.Is(dberrors.UserGroupErrors.ErrUniqueUserGroupsNameAccountIdKey, err) {
			zap.L().Warn(err.Error())
			return domain.UserGroup{}, ErrGroupNameExists
		}
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
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

// GetByID выбирает группы по идентификаторам без проверки прав — батч-выборка для
// внутренней сборки (например, профиля пользователя, §2.3 дизайна эпика Э2).
func (s *UserGroupService) GetByID(ctx context.Context, groupsID ...uuid.UUID) ([]domain.UserGroup, error) {
	groups, err := s.repo.GetByID(ctx, groupsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return groups, nil
}

// Get возвращает карточку одной группы (§3.2 дизайна эпика Э2, П-3). Право — любой участник
// аккаунта (IsHasUser, как список групп); группа не в аккаунте — ErrNotFound.
func (s *UserGroupService) Get(
	ctx context.Context,
	initiatorID, accountID, groupID uuid.UUID,
) (domain.UserGroup, error) {
	// Проверка, что инициатор является участником аккаунта
	if err := s.srv.Account.IsHasUser(ctx, accountID, initiatorID); err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	groups, err := s.repo.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			zap.L().Warn(err.Error())
			return domain.UserGroup{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	if groups[0].AccountID != accountID {
		return domain.UserGroup{}, ErrNotFound
	}

	return groups[0], nil
}

func (s *UserGroupService) Delete(
	ctx context.Context,
	initiatorID, accountID, groupID uuid.UUID,
) error {
	// Проверка прав на управление группами
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageGroups,
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Удаление группы каскадно: БД-транзакция, объекты хранилища каждого видео группы —
	// после коммита (Э1-Т21, §7.3 дизайна эпика).
	videoIDs, err := s.repo.DeleteCascade(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	s.srv.Video.DeleteObjectsAfterCommit(ctx, videoIDs...)

	return nil
}

// isAccessAddMembers проверяет право на добавление участников общей OR-логикой
// (Access.IsCheckGroupAction, §3.1 дизайна эпика Э2: право уровня аккаунта ManageGroups или
// Owner/ManageMembers в самой группе), а затем — что все добавляемые пользователи состоят в
// том же аккаунте, что и группа.
func (s *UserGroupService) isAccessAddMembers(
	ctx context.Context,
	accountID, initiatorID, groupID uuid.UUID,
	targetsID ...uuid.UUID,
) error {
	if err := s.srv.Access.IsCheckGroupAction(
		ctx,
		accountID, initiatorID, groupID,
		domain.AccountPermissionManageGroups, domain.GroupPermissionManageMembers,
	); err != nil {
		zap.L().Error(err.Error())
		return err
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

	return nil
}
