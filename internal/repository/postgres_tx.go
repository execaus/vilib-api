package repository

import (
	"context"
	"vilib-api/internal/saga"

	"github.com/stephenafamo/bob"
)

//go:generate mockgen -source=./postgres_tx.go -destination=./mocks/postgres_tx.go -package=mock_postgres

type ExecutorProvider struct {
	db *bob.DB
}

func NewTransactionalRepository(db *bob.DB) *ExecutorProvider {
	return &ExecutorProvider{db: db}
}

func (r *ExecutorProvider) WithTx(ctx context.Context) (bob.Transaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *ExecutorProvider) GetExecutor(ctx context.Context) bob.Executor {
	tx, ok := ctx.Value(saga.CtxKey).(bob.Transaction)
	if !ok {
		return r.db
	}

	return tx
}
