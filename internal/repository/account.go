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

func (r *AccountRepository) SelectByUsersID(ctx context.Context, usersID ...string) ([]domain.Account, error) {
	exec := r.provider.GetExecutor(ctx)

	accountsID := make([]uuid.UUID, len(usersID))

	for i, id := range usersID {
		permissionDB, err := schema.AccountPermissions.Query(
			sm.Where(schema.AccountPermissions.Columns.UserID.EQ(psql.S(id))),
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

func NewAccountRepository(provider *ExecutorProvider) *AccountRepository {
	return &AccountRepository{provider: provider}
}
