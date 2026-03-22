package service

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type AccountService struct {
	repo repository.Account
	srv  *Service
}

func NewAccountService(repo repository.Account, srv *Service) *AccountService {
	return &AccountService{repo: repo, srv: srv}
}

func (s *AccountService) Create(ctx context.Context, userName, userSurname, email string) (domain.Account, error) {
	// Вычисление имени аккаунта на основе прикрепленного email
	accountName, err := domain.NameFromEmail(email)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, ErrEmailInvalid
	}

	// Создание аккаунта
	account, err := s.repo.Insert(ctx, accountName, email)
	if err != nil {
		if errors.Is(err, dberrors.AccountErrors.ErrUniqueAccountsNameKey) {
			zap.L().Warn(err.Error())
			return account, ErrAccountNameExists
		}
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	// Генерация пароля для пользователя
	password, err := s.srv.Auth.GeneratePassword()
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	// Хеширование пароля
	passwordHash, err := s.srv.Auth.HashPassword(password)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	// Создание пользователя
	user, err := s.srv.User.Create(ctx, userName, userSurname, email, passwordHash)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	// Назначение пользователю статус супер администратора аккаунта
	if _, err = s.srv.AccountStatus.Issue(ctx, user.ID, domain.AccountSuperAdminBitPosition); err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	if err = s.srv.Email.SendRegisteredMail(ctx, user.Email, password); err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	return account, nil
}

func (s *AccountService) CreateUser(ctx context.Context, accountID, name, surname, email string) (domain.User, error) {
	// Существует ли пользователь в аккаунте
	exists, err := s.srv.Account.IsExistsUserByEmail(ctx, accountID, email)
	if exists {
		zap.L().Error(ErrAccountUserExists.Error())
		return domain.User{}, ErrAccountUserExists
	}

	// Генерация пароля для пользователя
	password, err := s.srv.Auth.GeneratePassword()
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	// Создание пользователя в базе данных
	user, err := s.srv.User.Create(ctx, name, surname, email, password)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	// Связывание пользователя с аккаунтом с правами обычного пользователя
	if _, err = s.srv.AccountStatus.Issue(ctx, user.ID, domain.AccountUserBitPosition); err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	// Отправка пароля новому пользователю на почту
	if err = s.srv.Email.SendCreateUserEmail(ctx, email, password); err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	return user, nil
}

func (s *AccountService) GetByUserEmail(ctx context.Context, email string) ([]domain.Account, error) {
	users, err := s.srv.User.GetByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	if len(users) == 0 {
		return nil, nil
	}

	usersID := make([]string, len(users))
	for i, user := range users {
		usersID[i] = user.ID
	}

	accountStatusesID, err := s.srv.AccountStatus.GetByUsersID(ctx, usersID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	accountsID := make([]string, len(accountStatusesID))
	for i, status := range accountStatusesID {
		accountsID[i] = status.AccountID
	}

	accounts, err := s.GetByID(ctx, accountsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return accounts, nil
}

func (s *AccountService) IsExistsUserByEmail(ctx context.Context, accountID, email string) (bool, error) {
	accounts, err := s.srv.Account.GetByUserEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return false, err
	}

	return slices.ContainsFunc(accounts, func(account domain.Account) bool {
		return account.ID == accountID
	}), nil
}

func (s *AccountService) GetByID(ctx context.Context, accountsID ...string) ([]domain.Account, error) {
	accounts, err := s.repo.SelectByID(ctx, accountsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return accounts, nil
}
