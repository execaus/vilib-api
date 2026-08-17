package service

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
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
	// Проверка прав доступа на создание роли группы — ManageGroups (§3.5 дизайна эпика Э2,
	// П-14): роли групп — часть управления группами, ManageRoles остаётся про роли аккаунта.
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageGroups,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	if isDefault {
		if err := s.repo.ClearDefault(ctx, accountID); err != nil {
			zap.L().Error(err.Error())
			return domain.GroupRole{}, err
		}
	}

	// Создание роли группы
	role, err := s.repo.Insert(ctx, accountID, name, permission, isDefault)
	if err != nil {
		if errors.Is(dberrors.GroupRoleErrors.ErrUniqueGroupRolesAccountIdNameKey, err) {
			zap.L().Warn(err.Error())
			return domain.GroupRole{}, ErrGroupRoleNameExists
		}
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	return role, nil
}

// Update редактирует роль группы — полная замена всех редактируемых полей (§4 дизайна эпика
// Э2, П-10). Право — ManageGroups; роль должна принадлежать accountID (иначе ErrNotFound — не
// раскрываем чужие роли); в отличие от ролей аккаунта бит GroupPermissionOwner в маске
// разрешён; назначение is_default=true снимает флаг у остальных ролей групп аккаунта в той же
// транзакции (ClearDefault); снятие is_default у текущей единственной дефолтной роли запрещено
// (ErrDefaultRoleRequired); дубль имени — ErrGroupRoleNameExists.
func (s *GroupRoleService) Update(
	ctx context.Context,
	initiatorID, accountID, roleID uuid.UUID,
	name string,
	mask domain.PermissionMask,
	isDefault bool,
) (domain.GroupRole, error) {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageGroups,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	roles, err := s.repo.SelectByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.GroupRole{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}
	if roles[0].AccountID != accountID {
		return domain.GroupRole{}, ErrNotFound
	}

	if !isDefault && roles[0].IsDefault {
		return domain.GroupRole{}, ErrDefaultRoleRequired
	}

	if isDefault {
		if clearErr := s.repo.ClearDefault(ctx, accountID); clearErr != nil {
			zap.L().Error(clearErr.Error())
			return domain.GroupRole{}, clearErr
		}
	}

	updated, err := s.repo.Update(ctx, roleID, name, mask, isDefault)
	if err != nil {
		if errors.Is(dberrors.GroupRoleErrors.ErrUniqueGroupRolesAccountIdNameKey, err) {
			zap.L().Warn(err.Error())
			return domain.GroupRole{}, ErrGroupRoleNameExists
		}
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	return updated, nil
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
		if errors.Is(err, repository.ErrNotFound) {
			zap.L().Warn(ErrDefaultGroupRoleNotFound.Error())
			return domain.GroupRole{}, ErrDefaultGroupRoleNotFound
		}
		zap.L().Error(err.Error())
		return domain.GroupRole{}, err
	}

	return role, nil
}

// GetAll возвращает список ролей групп аккаунта. Право — любой участник аккаунта (послабление
// §3.4 дизайна эпика Э2, П-7): менеджер группы (GroupPermissionManageMembers, без аккаунтных
// прав) должен выбрать role_id для смены роли участника (П-4); создание/удаление/правка
// по-прежнему требуют ManageGroups.
func (s *GroupRoleService) GetAll(
	ctx context.Context,
	initiatorID, accountID uuid.UUID,
) ([]domain.GroupRole, error) {
	if err := s.srv.Account.IsHasUser(ctx, accountID, initiatorID); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Получение всех ролей групп аккаунта
	roles, err := s.repo.SelectByAccount(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return roles, nil
}

// GetByAccountID выбирает все роли групп аккаунта без проверки прав — батч-выборка для
// сборки профиля пользователя (§2.3 дизайна эпика Э2): роль в собственных группах видна
// пользователю без права ManageGroups, требуемого GetAll.
func (s *GroupRoleService) GetByAccountID(ctx context.Context, accountID uuid.UUID) ([]domain.GroupRole, error) {
	roles, err := s.repo.SelectByAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return []domain.GroupRole{}, nil
		}
		zap.L().Error(err.Error())
		return nil, err
	}

	return roles, nil
}

func (s *GroupRoleService) Delete(
	ctx context.Context,
	initiatorID, accountID, roleID uuid.UUID,
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

	// Проверить, используется ли роль участниками группы
	members, err := s.repo.SelectMembersByRole(ctx, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if len(members) > 0 {
		return ErrGroupRoleInUse
	}

	// Удалить роль
	if err := s.repo.Delete(ctx, roleID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
