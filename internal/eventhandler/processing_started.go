package eventhandler

import (
	"context"
	"vilib-api/internal/service"

	"go.uber.org/zap"

	events "github.com/execaus/vilib-events"
)

// handleProcessingStarted обрабатывает событие ProcessingStarted внутри транзакции саги.
// Битая полезная нагрузка — poison message: логируется и не считается ошибкой обработки.
func (h *EventHandler) handleProcessingStarted(ctx context.Context, envelope events.Envelope) error {
	payload, err := envelope.ProcessingStarted()
	if err != nil {
		zap.L().Warn("failed to decode ProcessingStarted payload, skipping message",
			zap.String("event_id", envelope.EventID.String()),
			zap.Error(err),
		)
		return nil
	}

	return h.saga.Run(ctx, func(txCtx context.Context, services *service.Service) error {
		return services.Video.ApplyProcessingStarted(txCtx, envelope, payload)
	})
}
