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

	// Создание системной роли владельца аккаунта
	ownerRole, err := s.srv.AccountRole.CreateSystemAccountOwner(ctx, account.ID)
	if err != nil {
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
	user, err := s.srv.User.Create(ctx, userName, userSurname, email, passwordHash, ownerRole.ID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	// Отправка пароля на почту
	if err = s.srv.Email.SendRegisteredMail(ctx, user.Email, password); err != nil {
		zap.L().Error(err.Error())
		return domain.Account{}, err
	}

	return account, nil
}

func (s *AccountService) CreateUser(
	ctx context.Context,
	accountID uuid.UUID,
	name, surname, email string,
) (domain.User, error) {
	// Существует ли пользователь в аккаунте
	exists, err := s.srv.Account.IsExistsUserByEmail(ctx, email)
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

	// Получение дефолтной роли организации
	defaultRole, err := s.srv.AccountRole.GetDefault(ctx, accountID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.User{}, err
	}

	// Создание пользователя в базе данных
	user, err := s.srv.User.Create(ctx, name, surname, email, password, defaultRole.ID)
	if err != nil {
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
	// Получение пользователей с таким email
	users, err := s.srv.User.GetByEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Получение аккаунтов найденных пользователей
	usersID := make([]uuid.UUID, len(users))
	accountRolesID := make([]uuid.UUID, len(users))
	for i, user := range users {
		usersID[i] = user.ID
		accountRolesID[i] = user.RoleID
	}

	accountsID := make([]uuid.UUID, len(accountRolesID))
	accountsRole, err := s.srv.AccountRole.GetByID(ctx, accountRolesID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	for i, role := range accountsRole {
		accountsID[i] = role.ID
	}

	accounts, err := s.GetByID(ctx, accountsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return accounts, nil
}

func (s *AccountService) IsExistsUserByEmail(ctx context.Context, email string) (bool, error) {
	accounts, err := s.srv.Account.GetByUserEmail(ctx, email)
	if err != nil {
		zap.L().Error(err.Error())
		return false, err
	}

	for _, account := range accounts {
		if account.Email == email {
			return true, nil
		}
	}

	return false, nil
}

func (s *AccountService) GetByID(ctx context.Context, accountsID ...uuid.UUID) ([]domain.Account, error) {
	accounts, err := s.repo.SelectByID(ctx, accountsID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return accounts, nil
}
