package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
)

type UserService struct {
	repo repository.User
	srv  *Service
}

func NewUserService(repo repository.User, srv *Service) *UserService {
	return &UserService{repo: repo, srv: srv}
}

func (s *UserService) Create(ctx context.Context, name, surname, email, passwordHash string) (domain.User, error) {
	user, err := s.repo.Insert(ctx, name, surname, passwordHash, email)
	if err != nil {
		zap.L().Error(err.Error())
		return user, err
	}

	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) ([]domain.User, error) {
	users, err := s.repo.SelectByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return users, nil
}

func (s *UserService) Update(
	ctx context.Context,
	initiatorID, targetUserID string,
	status *domain.BitPosition,
) (domain.User, error) {
	var (
		initiatorStatus, targetStatus domain.BitPosition
		ok                            bool
	)

	// Проверка прав
	if status != nil {
		// Проверка, что инициатор и изменяемый пользователь не одно и то же лицо
		if initiatorID == targetUserID {
			return domain.User{}, ErrChangeAccountStatusConflict
		}

		// Проверка, что инициатор и изменяемый пользователь из одной организации
		accountStatuses, err := s.srv.AccountStatus.GetByUsersID(ctx, initiatorID, targetUserID)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}

		if accountStatuses[0].AccountID != accountStatuses[1].AccountID {
			return domain.User{}, ErrChangeAccountStatusForbidden
		}

		// Проверка наличия прав у инициатора изменять статус пользователя
		if initiatorStatus, ok = domain.HighestBitPosition(accountStatuses[0].Status); !ok {
			return domain.User{}, ErrInvalidStatus
		}
		if targetStatus, ok = domain.HighestBitPosition(accountStatuses[1].Status); !ok {
			return domain.User{}, ErrInvalidStatus
		}

		if initiatorStatus != domain.AccountSuperAdminBitPosition && initiatorStatus <= *status {
			return domain.User{}, ErrChangeAccountStatusForbidden
		}

		if initiatorStatus <= targetStatus {
			return domain.User{}, ErrChangeAccountStatusForbidden
		}
	}

	// Применение изменений
	if status != nil {
		_, err := s.srv.AccountStatus.Issue(ctx, targetUserID, *status)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.User{}, err
		}

		// Снижение уровня с супер администратора до администратора при передаче статуса супер администратора
		if initiatorStatus == domain.AccountSuperAdminBitPosition && *status == domain.AccountSuperAdminBitPosition {
			_, err = s.srv.AccountStatus.Issue(ctx, initiatorID, domain.AccountAdminBitPosition)
			if err != nil {
				zap.L().Error(err.Error())
				return domain.User{}, err
			}
		}
	}

	// Получение данных пользователя
	users, err := s.repo.GetByID(ctx, targetUserID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}
	if users == nil {
		zap.L().Error(ErrNotFound.Error())
		return domain.User{}, ErrNotFound
	}

	return users[0], nil
}
