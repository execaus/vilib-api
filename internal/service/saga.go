package service

import (
	"context"
	"vilib-api/internal/repository"
	"vilib-api/pkg"

	"go.uber.org/zap"
)

type SagaFunc = func(ctx context.Context, services *Service) error

type SagaRunner struct {
	service *Service
	repo    repository.Transactable
}

func NewSagaRunner(service *Service, repo repository.Transactable) SagaRunner {
	return SagaRunner{service: service, repo: repo}
}

func (r *SagaRunner) Run(ctx context.Context, fn SagaFunc) error {
	tx, err := r.repo.WithTx(ctx)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	sagaCtx := context.WithValue(ctx, pkg.SagaQueriesKey, tx)

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
