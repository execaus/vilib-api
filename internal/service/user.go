package service

import (
	"context"
	"errors"
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

// Update частично обновляет пользователя targetUserID в аккаунте accountID (§4 дизайна эпика
// Э2, «Блок C — редактирование»): смена роли или правка чужого профиля требуют ManageUsers;
// инициатор, правящий собственное ФИО без смены роли, — исключение без проверки прав (правка
// своего профиля). targetUserID должен состоять в accountID (роль пользователя принадлежит
// accountID), иначе — ErrNotFound. Пустой patch не меняет ничего и возвращает текущего
// пользователя.
func (s *UserService) Update(
	ctx context.Context,
	initiatorID, accountID, targetUserID uuid.UUID,
	patch domain.UserPatch,
) (domain.User, error) {
	// Само-правка ФИО без смены роли разрешена без прав; во всех остальных случаях требуется
	// ManageUsers.
	if patch.RoleID != nil || initiatorID != targetUserID {
		if err := s.srv.Access.IsCheckAccountAction(
			ctx,
			accountID,
			initiatorID,
			domain.AccountPermissionManageUsers,
		); err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}
	}

	// Целевой пользователь должен состоять в accountID (роль пользователя принадлежит
	// accountID) — иначе не раскрываем чужих пользователей (ErrNotFound).
	users, err := s.repo.SelectByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.User{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.User{}, err
	}
	target := users[0]

	targetRoles, err := s.srv.AccountRole.GetByID(ctx, target.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}
	if len(targetRoles) == 0 || targetRoles[0].AccountID != accountID {
		return domain.User{}, ErrNotFound
	}

	if patch.RoleID != nil {
		// Проверить, что новая роль принадлежит аккаунту
		var newRoles []domain.AccountRole
		newRoles, err = s.srv.AccountRole.GetByID(ctx, *patch.RoleID)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}
		if len(newRoles) == 0 {
			return domain.User{}, ErrNotFound
		}
		if newRoles[0].AccountID != accountID {
			return domain.User{}, ErrForbidden
		}

		target, err = s.repo.UpdateRole(ctx, targetUserID, *patch.RoleID)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}
	}

	if patch.Name != nil || patch.Surname != nil {
		target, err = s.repo.UpdateProfile(ctx, targetUserID, patch.Name, patch.Surname)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}
	}

	return target, nil
}

// GetByEmailAndAccountID возвращает строку пользователя с указанным email в указанной
// организации — используется переключением организации (§2.4 дизайна эпика Э2).
func (s *UserService) GetByEmailAndAccountID(
	ctx context.Context,
	email string,
	accountID uuid.UUID,
) (domain.User, error) {
	user, err := s.repo.SelectByEmailAndAccountID(ctx, email, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	return user, nil
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

// GetByIDs батчем выбирает пользователей по списку идентификаторов (П-6 контракта Э2: резолв
// авторов видео в списке). Отсутствие строки для части id — не ошибка.
func (s *UserService) GetByIDs(ctx context.Context, userID []uuid.UUID) ([]domain.User, error) {
	users, err := s.repo.SelectByIDs(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}

// UpdatePasswordHash обновляет хеш пароля одной строки пользователя userID (§6 дизайна эпика
// Э2, поправка О-1) — без проверки прав, вызывается только сервисом Auth для собственной или
// адресуемой токеном сброса строки.
func (s *UserService) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) (domain.User, error) {
	user, err := s.repo.UpdatePasswordHash(ctx, userID, hash)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	return user, nil
}

func (s *UserService) Deactivate(
	ctx context.Context,
	initiatorID, accountID, targetID uuid.UUID,
) error {
	// Проверка прав на управление пользователями
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageUsers,
	); err != nil {
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
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageUsers,
	); err != nil {
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
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageUsers,
	); err != nil {
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
