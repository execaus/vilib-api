package eventhandler

import (
	"context"
	"vilib-api/internal/service"

	"go.uber.org/zap"

	events "github.com/execaus/vilib-events"
)

// handleProcessingCompleted обрабатывает событие ProcessingCompleted внутри транзакции саги.
// Битая полезная нагрузка — poison message: логируется и не считается ошибкой обработки.
func (h *EventHandler) handleProcessingCompleted(ctx context.Context, envelope events.Envelope) error {
	payload, err := envelope.ProcessingCompleted()
	if err != nil {
		zap.L().Warn("failed to decode ProcessingCompleted payload, skipping message",
			zap.String("event_id", envelope.EventID.String()),
			zap.Error(err),
		)
		return nil
	}

	return h.saga.Run(ctx, func(txCtx context.Context, services *service.Service) error {
		return services.Video.ApplyProcessingCompleted(txCtx, envelope, payload)
	})
}
