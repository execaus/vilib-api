package service

import (
	"context"
	"errors"
	"strings"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/gen/schema"
	"vilib-api/internal/repository"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

type AccountService struct {
	repo *repository.TransactionalRepository
}

func NewAccountService(repo *repository.TransactionalRepository) *AccountService {
	return &AccountService{repo: repo}
}

func (s *AccountService) Create(ctx context.Context, email string) (domain.Account, error) {
	exec := s.repo.GetExecutor(ctx)

	var account domain.Account

	var accountName string
	if i := strings.Index(email, "@"); i != -1 {
		accountName = email[:i]
	} else {
		return account, ErrEmailInvalid
	}

	accountDB, err := schema.Accounts.Insert(&schema.AccountSetter{
		Name:  omit.From(accountName),
		Email: omit.From(email),
	}).One(ctx, exec)
	if err != nil {
		if errors.Is(err, dberrors.AccountErrors.ErrUniqueAccountsNameKey) {
			zap.L().Warn(err.Error())
			return account, ErrAccountNameExists
		}
		zap.L().Error(err.Error())
		return account, err
	}

	account.FromDB(accountDB)

	return account, nil
}

func (s *AccountService) GetByUserEmail(ctx context.Context, email string) ([]domain.Account, error) {
	exec := s.repo.GetExecutor(ctx)

	usersDB, err := schema.Users.Query(
		sm.Where(schema.Users.Columns.Email.EQ(psql.S(email))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return []domain.Account{}, nil
	}

	if len(usersDB) == 0 {
		return []domain.Account{}, nil
	}

	accountsID := make([]uuid.UUID, len(usersDB))

	for i, user := range usersDB {
		permissionDB, err := schema.AccountPermissions.Query(
			sm.Where(schema.AccountPermissions.Columns.UserID.EQ(psql.Arg(user.UserID))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return []domain.Account{}, nil
		}

		accountsID[i] = permissionDB.AccountID
	}

	accounts := make([]domain.Account, len(accountsID))
	for i, accountID := range accountsID {
		accountDB, err := schema.Accounts.Query(
			sm.Where(schema.Accounts.Columns.AccountID.EQ(psql.Arg(accountID))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return []domain.Account{}, err
		}

		accounts[i] = domain.Account{}
		accounts[i].FromDB(accountDB)
	}

	return accounts, nil
}
