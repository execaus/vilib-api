package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

type AccountRepository struct {
	provider *ExecutorProvider
}

func NewAccountRepository(provider *ExecutorProvider) *AccountRepository {
	return &AccountRepository{provider: provider}
}

func (r *AccountRepository) SelectByUsersID(ctx context.Context, usersID ...uuid.UUID) ([]domain.Account, error) {
	exec := r.provider.GetExecutor(ctx)

	rolesID := make([]uuid.UUID, len(usersID))

	// Получение уникальных значений ролей
	for i, id := range usersID {
		userDB, err := schema.Users.Query(
			sm.Where(schema.Users.Columns.UserID.EQ(psql.Arg(id))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		rolesID[i] = userDB.RoleID
	}

	// Получение уникальных значений аккаунтов
	accountsID := make(map[uuid.UUID]struct{}, len(rolesID))
	for _, v := range rolesID {
		accountRole, err := schema.AccountRoles.Query(
			sm.Where(schema.AccountRoles.Columns.AccountRoleID.EQ(psql.Arg(v))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}
		accountsID[accountRole.AccountID] = struct{}{}
	}

	accounts := make([]domain.Account, 0, len(accountsID))
	for accountID, _ := range accountsID {
		accountDB, err := schema.Accounts.Query(
			sm.Where(schema.Accounts.Columns.AccountID.EQ(psql.Arg(accountID))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return []domain.Account{}, err
		}

		account := domain.Account{}
		account.FromDB(accountDB)

		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (r *AccountRepository) Insert(ctx context.Context, name, email string) (domain.Account, error) {
	exec := r.provider.GetExecutor(ctx)

	var account domain.Account

	accountDB, err := schema.Accounts.Insert(&schema.AccountSetter{
		Name:  omit.From(name),
		Email: omit.From(email),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return account, err
	}

	account.FromDB(accountDB)

	return account, nil
}

func (r *AccountRepository) SelectByID(ctx context.Context, accountsID ...uuid.UUID) ([]domain.Account, error) {
	exec := r.provider.GetExecutor(ctx)

	accounts := make([]domain.Account, len(accountsID))

	for i, id := range accountsID {
		accounts[i] = domain.Account{}

		accountDB, err := schema.Accounts.Query(
			sm.Where(schema.Accounts.Columns.AccountID.EQ(psql.Arg(id))),
		).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		if accountDB == nil {
			zap.L().Warn("account not found: " + id.String())
			return nil, nil
		}

		accounts[i].FromDB(accountDB)
	}

	return accounts, nil
}
