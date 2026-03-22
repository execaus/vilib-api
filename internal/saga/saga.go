package saga

import (
	"context"

	"go.uber.org/zap"
)

const (
	CtxKey = "saga-queries-key"
)

type Func[ServiceT any] = func(ctx context.Context, services ServiceT) error

type Runner[ServiceT any] struct {
	service ServiceT
	repo    Transactable
}

func NewSagaRunner[ServiceT any](service ServiceT, repo Transactable) Runner[ServiceT] {
	return Runner[ServiceT]{service: service, repo: repo}
}

func (r *Runner[ServiceT]) Run(ctx context.Context, fn Func[ServiceT]) error {
	tx, err := r.repo.WithTx(ctx)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	sagaCtx := context.WithValue(ctx, CtxKey, tx)

	if err = fn(sagaCtx, r.service); err != nil {
		zap.L().Error(err.Error())
		if err := tx.Rollback(ctx); err != nil {
			zap.L().Error(err.Error())
		}
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
