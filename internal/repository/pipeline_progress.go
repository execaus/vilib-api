package repository

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

// PipelineProgressRepository — реализация репозитория индикатора живости конвейера обработки
// видео по полосам (эпик Э5, исправление Д-1).
type PipelineProgressRepository struct {
	provider *ExecutorProvider
}

// NewPipelineProgressRepository создаёт репозиторий индикатора прогресса конвейера.
func NewPipelineProgressRepository(provider *ExecutorProvider) *PipelineProgressRepository {
	return &PipelineProgressRepository{provider: provider}
}

// Select выбирает индикатор прогресса указанной полосы обработки.
func (r *PipelineProgressRepository) Select(ctx context.Context, isUrgent bool) (domain.PipelineProgress, error) {
	exec := r.provider.GetExecutor(ctx)

	row, err := schema.PipelineProgresses.Query(
		sm.Where(schema.PipelineProgresses.Columns.IsUrgent.EQ(psql.Arg(isUrgent))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.PipelineProgress{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.PipelineProgress{}, err
	}

	var progress domain.PipelineProgress
	progress.FromDB(row)

	return progress, nil
}

// UpdateLastDequeuedAt обновляет момент последнего успешного взятия видео в обработку указанной
// полосой — вызывается в той же транзакции, что переводит видео queued → compressing по
// событию ProcessingStarted (§1 дизайна эпика Э5, исправление Д-1).
func (r *PipelineProgressRepository) UpdateLastDequeuedAt(ctx context.Context, isUrgent bool, now time.Time) error {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.PipelineProgressSetter{LastDequeuedAt: omit.From(now)}

	_, err := schema.PipelineProgresses.Update(
		setter.UpdateMod(),
		um.Where(schema.PipelineProgresses.Columns.IsUrgent.EQ(psql.Arg(isUrgent))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
