package repository

import (
	"context"
	"encoding/json"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/types"
	"go.uber.org/zap"
)

type OutboxRepository struct {
	provider *ExecutorProvider
}

func NewOutboxRepository(provider *ExecutorProvider) *OutboxRepository {
	return &OutboxRepository{provider: provider}
}

// Insert кладёт событие в очередь публикации в рамках текущей транзакции (та же, что и
// основная запись саги) — это и есть transactional outbox.
func (r *OutboxRepository) Insert(ctx context.Context, topic, key string, payload []byte) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.OutboxEvents.Insert(&schema.OutboxEventSetter{
		Topic:   omit.From(topic),
		Key:     omit.From(key),
		Payload: omit.From(types.NewJSON(json.RawMessage(payload))),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// SelectBatchForUpdate выбирает до limit самых старых событий очереди и блокирует их
// (FOR UPDATE SKIP LOCKED) — параллельные вызовы (в т. ч. из нескольких инстансов релея)
// не получают одни и те же строки.
func (r *OutboxRepository) SelectBatchForUpdate(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	exec := r.provider.GetExecutor(ctx)

	eventsDB, err := schema.OutboxEvents.Query(
		sm.OrderBy(schema.OutboxEvents.Columns.ID),
		sm.Limit(limit),
		sm.ForUpdate().SkipLocked(),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	events := make([]domain.OutboxEvent, len(eventsDB))
	for i, e := range eventsDB {
		events[i].FromDB(e)
	}

	return events, nil
}

// DeleteByIDs удаляет опубликованные события очереди. Пустой список ids — no-op.
func (r *OutboxRepository) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	exec := r.provider.GetExecutor(ctx)

	args := make([]bob.Expression, len(ids))
	for i, id := range ids {
		args[i] = psql.Arg(id)
	}

	_, err := schema.OutboxEvents.Delete(
		dm.Where(schema.OutboxEvents.Columns.ID.In(args...)),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
