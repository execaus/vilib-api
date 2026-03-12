package repository

import (
	"context"
	"vilib-api/pkg"

	"github.com/stephenafamo/bob"
)

//go:generate mockgen -source=./postgres_tx.go -destination=./mocks/postgres_tx.go -package=mock_postgres
type Transactable interface {
	WithTx(ctx context.Context) (bob.Transaction, error)
}

type BobTransaction interface {
	bob.Transaction
}

type TransactionalRepository struct {
	db *bob.DB
}

func NewTransactionalRepository(db *bob.DB) *TransactionalRepository {
	return &TransactionalRepository{db: db}
}

func (r *TransactionalRepository) WithTx(ctx context.Context) (bob.Transaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *TransactionalRepository) GetExecutor(ctx context.Context) bob.Executor {
	tx, ok := ctx.Value(pkg.SagaQueriesKey).(bob.Transaction)
	if !ok {
		return r.db
	}

	return tx
}
