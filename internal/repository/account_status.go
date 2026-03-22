package repository

import (
	"context"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

type AccountStatusRepository struct {
	provider *ExecutorProvider
}

func NewAccountStatusRepository(provider *ExecutorProvider) *AccountStatusRepository {
	return &AccountStatusRepository{provider: provider}
}

func (r *AccountStatusRepository) Upsert(
	ctx context.Context,
	userID, accountID string,
	status domain.BitmapValue,
) error {
	exec := r.provider.GetExecutor(ctx)

	accountStatusDB, err := schema.AccountStatuses.Query(
		sm.Where(schema.AccountStatuses.Columns.UserID.EQ(psql.S(userID))),
		sm.Where(schema.AccountStatuses.Columns.AccountID.EQ(psql.S(accountID))),
	).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if accountStatusDB == nil {
		_, err = schema.AccountStatuses.Insert(&schema.AccountStatusSetter{
			UserID:    omit.From(uuid.MustParse(userID)),
			AccountID: omit.From(uuid.MustParse(accountID)),
			Status:    omit.From(status),
			UpdatedAt: omit.From(time.Now()),
		}).Exec(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}
	} else {
		_, err = schema.AccountStatuses.Update(
			um.SetCol(schema.AccountStatuses.Columns.Status.String()).ToArg(status),
			um.SetCol(schema.AccountStatuses.Columns.UpdatedAt.String()).ToArg(time.Now()),
			um.Where(schema.AccountStatuses.Columns.UserID.EQ(psql.Arg(userID))),
			um.Where(schema.AccountStatuses.Columns.AccountID.EQ(psql.Arg(accountID))),
		).Exec(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}
	}

	return nil
}
