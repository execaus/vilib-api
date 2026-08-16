package saga

import (
	"context"

	"go.uber.org/zap"
)

// ctxKey — приватный тип ключей контекста саги: защищает от коллизий с ключами контекста
// других пакетов при использовании [context.WithValue].
type ctxKey string

const (
	CtxKey ctxKey = "saga-queries-key"
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

	hooks := &Hooks{}
	sagaCtx := context.WithValue(ctx, CtxKey, tx)
	sagaCtx = context.WithValue(sagaCtx, HooksCtxKey, hooks)

	if err = fn(sagaCtx, r.service); err != nil {
		zap.L().Error(err.Error())
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			zap.L().Error(rollbackErr.Error())
		}
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Хуки выполняются с исходным ctx (без транзакции в контексте): транзакция уже
	// закоммичена, работа через ExecutorProvider должна идти мимо неё.
	hooks.run(ctx)

	return nil
}
