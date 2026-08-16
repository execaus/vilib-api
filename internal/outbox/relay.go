// Package outbox содержит фоновый релей transactional outbox (§7.1 эпика): периодически
// вычитывает события из очереди app.outbox_events и публикует их через kafka.Producer.
package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
	"vilib-api/internal/kafka"
	"vilib-api/internal/repository"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"go.uber.org/zap"
)

// headerEventType/headerEventVersion — заголовки Kafka, по которым консьюмеры дёшево
// фильтруют сообщения без разбора тела (§6.2 эпика).
const (
	headerEventType    = "event-type"
	headerEventVersion = "event-version"

	// envelopeHeaderCount — число заголовков, которые извлекаются из конверта (event-type,
	// event-version) — используется для предварительного выделения карты заголовков.
	envelopeHeaderCount = 2
)

// Relay — фоновый релей очереди outbox_events: каждые interval выбирает до batchSize
// старейших событий (FOR UPDATE SKIP LOCKED), публикует их через producer и удаляет из
// очереди — всё внутри одной транзакции саги. Ошибка публикации откатывает транзакцию:
// строки остаются в очереди и будут опубликованы повторно на следующем тике (at-least-once).
type Relay struct {
	runner    saga.Runner[*service.Service]
	repo      repository.Outbox
	producer  kafka.Producer
	interval  time.Duration
	batchSize int
	logger    *zap.Logger
}

// NewRelay создаёt релей очереди outbox_events.
func NewRelay(
	runner saga.Runner[*service.Service],
	repo repository.Outbox,
	producer kafka.Producer,
	interval time.Duration,
	batchSize int,
	logger *zap.Logger,
) *Relay {
	return &Relay{
		runner:    runner,
		repo:      repo,
		producer:  producer,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger,
	}
}

// Run блокируется до отмены ctx, выполняя тики релея с периодом interval. Если очередной
// батч оказался полным (== batchSize), следующий тик выполняется немедленно — так очередь
// не накапливается в периоды всплеска нагрузки.
func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.drain(ctx)
		}
	}
}

// drain выполняет тики подряд, пока очередной батч выбирается полным, либо до ошибки или
// отмены ctx — так релей быстро опустошает накопившуюся очередь, не дожидаясь interval.
func (r *Relay) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		full, err := r.tick(ctx)
		if err != nil {
			r.logger.Error("outbox relay tick failed", zap.Error(err))
			return
		}

		if !full {
			return
		}
	}
}

// tick выполняет один проход релея внутри транзакции саги: выборка батча, публикация
// каждого события, удаление опубликованных. Возвращает true, если выбранный батч был
// полным (== batchSize) — сигнал вызвать следующий тик немедленно.
func (r *Relay) tick(ctx context.Context) (bool, error) {
	full := false

	err := r.runner.Run(ctx, func(txCtx context.Context, _ *service.Service) error {
		events, selectErr := r.repo.SelectBatchForUpdate(txCtx, r.batchSize)
		if selectErr != nil {
			return fmt.Errorf("select outbox batch: %w", selectErr)
		}

		if len(events) == 0 {
			return nil
		}

		full = len(events) == r.batchSize

		ids := make([]int64, 0, len(events))
		for _, event := range events {
			headers := extractHeaders(event.Payload)

			if pubErr := r.producer.Publish(txCtx, event.Topic, event.Key, event.Payload, headers); pubErr != nil {
				return fmt.Errorf("publish outbox event %d to topic %s: %w", event.ID, event.Topic, pubErr)
			}

			ids = append(ids, event.ID)
		}

		if delErr := r.repo.DeleteByIDs(txCtx, ids); delErr != nil {
			return fmt.Errorf("delete published outbox events: %w", delErr)
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return full, nil
}

// envelopeHeaderFields — верхнеуровневые поля конверта события (§6.2 эпика), нужные только
// для заполнения заголовков Kafka. Полный разбор payload релею не нужен и невозможен — типы
// событий (vilib-events) он не импортирует, чтобы оставаться агностичным к их набору.
type envelopeHeaderFields struct {
	EventType string      `json:"event_type"`
	Version   json.Number `json:"version"`
}

// extractHeaders пытается извлечь event_type/version из верхнего уровня JSON-конверта payload
// и превратить их в заголовки Kafka event-type/event-version (дёшево фильтровать на стороне
// консьюмера без разбора тела). Если payload не JSON или поля отсутствуют — заголовки не
// выставляются (Publish получает nil/пустую карту), сама публикация при этом не прерывается:
// заголовки — оптимизация, а не обязательная часть контракта.
func extractHeaders(payload []byte) map[string]string {
	var fields envelopeHeaderFields

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return nil
	}

	headers := make(map[string]string, envelopeHeaderCount)
	if fields.EventType != "" {
		headers[headerEventType] = fields.EventType
	}
	if fields.Version != "" {
		headers[headerEventVersion] = fields.Version.String()
	}

	if len(headers) == 0 {
		return nil
	}

	return headers
}
