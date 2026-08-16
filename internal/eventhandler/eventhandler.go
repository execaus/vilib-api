// Package eventhandler содержит обработчик событий воркера обработки видео
// (топик video.processing-events, §7.2 эпика): диспетчер поверх kafka.Consumer.Run без gin,
// по аналогии с internal/handler.
package eventhandler

import (
	"context"
	"vilib-api/internal/kafka"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"go.uber.org/zap"

	events "github.com/execaus/vilib-events"
)

// EventHandler декодирует сообщения Kafka топика video.processing-events и выполняет
// обработчик соответствующего события внутри транзакции саги.
type EventHandler struct {
	saga saga.Runner[*service.Service]
}

// NewEventHandler создаёт обработчика событий поверх раннера саги.
func NewEventHandler(saga saga.Runner[*service.Service]) *EventHandler {
	return &EventHandler{saga: saga}
}

// Handle декодирует конверт события и диспетчеризует его по типу. Ошибка декодирования
// конверта, неподдерживаемая версия контракта и неизвестный тип события считаются poison
// message: логируются и не возвращаются — kafka.Consumer закоммитит offset сообщения.
// Ошибка обработчика (сбой БД/S3) возвращается вызывающему — offset не коммитится, сообщение
// повторяется с backoff'ом (Э1-Т14, §7.2 эпика).
func (h *EventHandler) Handle(ctx context.Context, msg kafka.Message) error {
	envelope, err := events.Unmarshal(msg.Value)
	if err != nil {
		zap.L().Warn("failed to decode kafka event envelope, skipping message", zap.Error(err))
		return nil
	}

	switch envelope.EventType {
	case events.TypeProcessingStarted:
		return h.handleProcessingStarted(ctx, envelope)
	case events.TypeProcessingCompleted:
		return h.handleProcessingCompleted(ctx, envelope)
	case events.TypeProcessingFailed:
		return h.handleProcessingFailed(ctx, envelope)
	default:
		zap.L().Warn("unknown event type, skipping message",
			zap.String("event_type", envelope.EventType),
			zap.String("event_id", envelope.EventID.String()),
		)
		return nil
	}
}
