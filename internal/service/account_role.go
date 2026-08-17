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

type AccountRoleService struct {
	repo repository.AccountRole
	srv  *Service
}

func NewAccountRoleService(repo repository.AccountRole, srv *Service) *AccountRoleService {
	return &AccountRoleService{repo: repo, srv: srv}
}

func (s *AccountRoleService) GetDefault(ctx context.Context, accountID uuid.UUID) (domain.AccountRole, error) {
	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

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
	permission := domain.SetBits(domain.DefaultPermissionMask, domain.AccountPermissionOwner)

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
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageRoles,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	// Бит владельца назначается только системной ролью, созданной при регистрации аккаунта
	// (CreateSystemAccountOwner) — вручную через Create/Update запрещено (§4 дизайна эпика Э2).
	if domain.HasBit(permission, domain.AccountPermissionOwner) {
		return domain.AccountRole{}, ErrPermissionOwnerForbidden
	}

	if isDefault {
		if err := s.repo.ClearDefault(ctx, accountID); err != nil {
			zap.L().Error(err.Error())
			return domain.AccountRole{}, err
		}
	}

	if _, err := s.repo.Insert(ctx, accountID, name, parentID, permission, isDefault, false); err != nil {
		if errors.Is(dberrors.AccountRoleErrors.ErrUniqueUniqueAccountRole, err) {
			zap.L().Warn(err.Error())
			return domain.AccountRole{}, ErrAccountRoleNameExists
		}
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	return roles[0], nil
}

// Update редактирует роль аккаунта — полная замена всех редактируемых полей (§4 дизайна эпика
// Э2, П-10). Право — ManageRoles; роль должна принадлежать accountID (иначе ErrNotFound — не
// раскрываем чужие роли); системную роль редактировать нельзя (ErrIsSystemRole); бит
// AccountPermissionOwner в маске запрещён (ErrPermissionOwnerForbidden); назначение
// is_default=true снимает флаг у остальных ролей аккаунта в той же транзакции (ClearDefault);
// снятие is_default у текущей единственной дефолтной роли запрещено (ErrDefaultRoleRequired);
// дубль имени — ErrAccountRoleNameExists.
func (s *AccountRoleService) Update(
	ctx context.Context,
	initiatorID, accountID, roleID uuid.UUID,
	name string,
	parentID *uuid.UUID,
	mask domain.PermissionMask,
	isDefault bool,
) (domain.AccountRole, error) {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageRoles,
	); err != nil {
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	roles, err := s.repo.SelectByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.AccountRole{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}
	if roles[0].AccountID != accountID {
		return domain.AccountRole{}, ErrNotFound
	}

	if roles[0].IsSystem {
		return domain.AccountRole{}, ErrIsSystemRole
	}

	if domain.HasBit(mask, domain.AccountPermissionOwner) {
		return domain.AccountRole{}, ErrPermissionOwnerForbidden
	}

	if !isDefault && roles[0].IsDefault {
		return domain.AccountRole{}, ErrDefaultRoleRequired
	}

	if isDefault {
		if clearErr := s.repo.ClearDefault(ctx, accountID); clearErr != nil {
			zap.L().Error(clearErr.Error())
			return domain.AccountRole{}, clearErr
		}
	}

	updated, err := s.repo.Update(ctx, roleID, name, parentID, mask, isDefault)
	if err != nil {
		if errors.Is(dberrors.AccountRoleErrors.ErrUniqueUniqueAccountRole, err) {
			zap.L().Warn(err.Error())
			return domain.AccountRole{}, ErrAccountRoleNameExists
		}
		zap.L().Error(err.Error())
		return domain.AccountRole{}, err
	}

	return updated, nil
}

// GetAll возвращает список ролей аккаунта. Право — ManageRoles или ManageUsers (послабление
// §3.4 дизайна эпика Э2, П-7): список ролей (имя+маска) — справочник, нужный, например, HR для
// колонки «роль» и выпадающего списка при смене роли пользователя; давать HR ManageRoles ради
// чтения — избыточное право.
func (s *AccountRoleService) GetAll(
	ctx context.Context,
	initiatorID, accountID uuid.UUID,
) ([]domain.AccountRole, error) {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageRoles,
	); err != nil {
		if !errors.Is(err, ErrForbidden) {
			zap.L().Error(err.Error())
			return nil, err
		}

		if manageUsersErr := s.srv.Access.IsCheckAccountAction(
			ctx,
			accountID,
			initiatorID,
			domain.AccountPermissionManageUsers,
		); manageUsersErr != nil {
			return nil, manageUsersErr
		}
	}

	roles, err := s.repo.SelectByAccountID(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return roles, nil
}

func (s *AccountRoleService) Delete(
	ctx context.Context,
	initiatorID, accountID, roleID uuid.UUID,
) error {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageRoles,
	); err != nil {
		return err
	}

	roles, err := s.repo.SelectByID(ctx, roleID)
	if err != nil || len(roles) == 0 {
		return ErrNotFound
	}

	if roles[0].IsSystem {
		return ErrIsSystemRole
	}

	activeUsers, err := s.repo.SelectActiveUsersByRole(ctx, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if len(activeUsers) > 0 {
		return ErrRoleInUse
	}

	if err := s.repo.Delete(ctx, roleID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
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
	roles, err := s.repo.SelectByID(ctx, rolesID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return roles, nil
}
