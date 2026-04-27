package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserService struct {
	repo repository.User
	srv  *Service
}

func NewUserService(repo repository.User, srv *Service) *UserService {
	return &UserService{repo: repo, srv: srv}
}

func (s *UserService) Create(
	ctx context.Context,
	name, surname, email, passwordHash string,
	roleID uuid.UUID,
) (domain.User, error) {
	// Создание пользователя в базе данных
	user, err := s.repo.Insert(ctx, name, surname, passwordHash, email, roleID)
	if err != nil {
		zap.L().Error(err.Error())
		return user, err
	}

	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) ([]domain.User, error) {
	// Получение пользователей с таким email
	users, err := s.repo.SelectByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}

func (s *UserService) Update(
	ctx context.Context,
	initiatorID, accountID, targetUserID uuid.UUID,
	roleID *uuid.UUID,
) (domain.User, error) {
	// Проверка прав на управление пользователями
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageUsers); err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	if roleID != nil {
		// Проверить, что роль принадлежит аккаунту
		roles, err := s.srv.AccountRole.GetByID(ctx, *roleID)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}
		if len(roles) == 0 {
			return domain.User{}, ErrNotFound
		}
		if roles[0].AccountID != accountID {
			return domain.User{}, ErrForbidden
		}

		// Обновить роль пользователя
		user, err := s.repo.UpdateRole(ctx, targetUserID, *roleID)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}
		return user, nil
	}

	// Если roleID == nil — просто получить текущего пользователя
	users, err := s.repo.SelectByID(ctx, targetUserID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}
	if len(users) == 0 {
		return domain.User{}, ErrNotFound
	}

	return users[0], nil
}

func (s *UserService) GetByID(ctx context.Context, userID ...uuid.UUID) ([]domain.User, error) {
	// Получение пользователей по ID
	users, err := s.repo.SelectByID(ctx, userID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}

func (s *UserService) Deactivate(
	ctx context.Context,
	initiatorID, accountID, targetID uuid.UUID,
) error {
	// Проверка прав на управление пользователями
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageUsers); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Получение целевого пользователя
	users, err := s.repo.SelectByID(ctx, targetID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if len(users) == 0 {
		return ErrNotFound
	}
	user := users[0]

	// Проверка, что пользователь активен
	if !user.IsActive() {
		return ErrUserDeactivated
	}

	// Проверка, что пользователь не является владельцем
	roles, err := s.srv.AccountRole.GetByID(ctx, user.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if len(roles) > 0 && roles[0].IsSystem {
		return ErrIsOwner
	}

	// Деактивация пользователя
	if err := s.repo.Deactivate(ctx, targetID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

func (s *UserService) Reactivate(
	ctx context.Context,
	initiatorID, accountID, targetID uuid.UUID,
) error {
	// Проверка прав на управление пользователями
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageUsers); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Получение целевого пользователя
	users, err := s.repo.SelectByID(ctx, targetID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}
	if len(users) == 0 {
		return ErrNotFound
	}
	user := users[0]

	// Проверка, что пользователь деактивирован
	if user.IsActive() {
		return ErrUserAlreadyActive
	}

	// Реактивация пользователя
	if err := s.repo.Reactivate(ctx, targetID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Проверка, что роль существует; если нет — назначить дефолтную
	roles, err := s.srv.AccountRole.GetByID(ctx, user.RoleID)
	if err != nil || len(roles) == 0 {
		defaultRole, err := s.srv.AccountRole.GetDefault(ctx, accountID)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}
		if _, err := s.repo.UpdateRole(ctx, targetID, defaultRole.ID); err != nil {
			zap.L().Error(err.Error())
			return err
		}
	}

	return nil
}

func (s *UserService) ListByAccount(
	ctx context.Context,
	initiatorID, accountID uuid.UUID,
	status repository.UserStatus,
) ([]domain.User, error) {
	// Проверка прав на управление пользователями
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageUsers); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Получение списка пользователей аккаунта
	users, err := s.repo.SelectByAccountID(ctx, accountID, status)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}
