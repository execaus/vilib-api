package eventhandler_test

import (
	"context"
	"errors"
	"testing"
	"vilib-api/internal/eventhandler"
	"vilib-api/internal/kafka"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	events "github.com/execaus/vilib-events"
)

// newTestEventHandler собирает EventHandler поверх раннера саги с моком видеосервиса и
// настраиваемым результатом Commit транзакции (по аналогии с testutil.SetupTestRouterWithMocks
// и internal/saga/hooks_test.go newRunnerWithTx).
func newTestEventHandler(
	t *testing.T,
	mc *minimock.Controller,
	videoMock *service_mocks.VideoMock,
) *eventhandler.EventHandler {
	t.Helper()

	tx := saga_mocks.NewBobTransactionMock(mc)
	tx.CommitMock.Return(nil)
	tx.CommitMock.Optional()
	tx.RollbackMock.Return(nil)
	tx.RollbackMock.Optional()

	repo := saga_mocks.NewTransactableMock(mc)
	repo.WithTxMock.Return(tx, nil)
	repo.WithTxMock.Optional()

	runner := saga.NewSagaRunner(&service.Service{Video: videoMock}, repo)

	return eventhandler.NewEventHandler(runner)
}

func TestEventHandler_Handle_RoutesEventsByType(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()

	tests := []struct {
		name          string
		buildEnvelope func() (events.Envelope, error)
		setupMock     func(m *service_mocks.VideoMock)
	}{
		{
			name: "ProcessingStarted",
			buildEnvelope: func() (events.Envelope, error) {
				return events.NewProcessingStarted(testVideoID, 1, events.ProcessingStarted{WorkerID: "worker-1"})
			},
			setupMock: func(m *service_mocks.VideoMock) {
				m.ApplyProcessingStartedMock.Set(func(
					_ context.Context, evt events.Envelope, p events.ProcessingStarted,
				) error {
					require.Equal(t, events.TypeProcessingStarted, evt.EventType)
					require.Equal(t, "worker-1", p.WorkerID)
					return nil
				})
			},
		},
		{
			name: "ProcessingCompleted",
			buildEnvelope: func() (events.Envelope, error) {
				return events.NewProcessingCompleted(testVideoID, 1, events.ProcessingCompleted{WorkerID: "worker-1"})
			},
			setupMock: func(m *service_mocks.VideoMock) {
				m.ApplyProcessingCompletedMock.Set(func(
					_ context.Context, evt events.Envelope, p events.ProcessingCompleted,
				) error {
					require.Equal(t, events.TypeProcessingCompleted, evt.EventType)
					require.Equal(t, "worker-1", p.WorkerID)
					return nil
				})
			},
		},
		{
			name: "ProcessingFailed",
			buildEnvelope: func() (events.Envelope, error) {
				return events.NewProcessingFailed(testVideoID, 1, events.ProcessingFailed{
					WorkerID: "worker-1", ErrorClass: events.ErrorClassPermanent, Reason: "bad file",
				})
			},
			setupMock: func(m *service_mocks.VideoMock) {
				m.ApplyProcessingFailedMock.Set(func(
					_ context.Context, evt events.Envelope, p events.ProcessingFailed,
				) error {
					require.Equal(t, events.TypeProcessingFailed, evt.EventType)
					require.Equal(t, "bad file", p.Reason)
					return nil
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)
			videoMock := service_mocks.NewVideoMock(mc)
			tt.setupMock(videoMock)

			h := newTestEventHandler(t, mc, videoMock)

			envelope, err := tt.buildEnvelope()
			require.NoError(t, err)
			payload, err := envelope.Marshal()
			require.NoError(t, err)

			err = h.Handle(t.Context(), kafka.Message{Value: payload})

			require.NoError(t, err)
		})
	}
}

func TestEventHandler_Handle_ApplyErrorPropagates(t *testing.T) {
	t.Parallel()

	testVideoID := uuid.New()
	applyErr := errors.New("db unavailable")

	mc := minimock.NewController(t)
	videoMock := service_mocks.NewVideoMock(mc)
	videoMock.ApplyProcessingStartedMock.Return(applyErr)

	h := newTestEventHandler(t, mc, videoMock)

	envelope, err := events.NewProcessingStarted(testVideoID, 1, events.ProcessingStarted{WorkerID: "worker-1"})
	require.NoError(t, err)
	payload, err := envelope.Marshal()
	require.NoError(t, err)

	err = h.Handle(t.Context(), kafka.Message{Value: payload})

	require.ErrorIs(t, err, applyErr)
}

func TestEventHandler_Handle_InvalidJSON_ReturnsNil(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)
	videoMock := service_mocks.NewVideoMock(mc)

	h := newTestEventHandler(t, mc, videoMock)

	err := h.Handle(t.Context(), kafka.Message{Value: []byte(`{"event_type":`)})

	require.NoError(t, err)
}

func TestEventHandler_Handle_UnknownEventType_ReturnsNil(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)
	videoMock := service_mocks.NewVideoMock(mc)

	h := newTestEventHandler(t, mc, videoMock)

	payload := []byte(`{"event_type":"SomethingElse","video_id":"` + uuid.New().String() + `","version":1}`)

	err := h.Handle(t.Context(), kafka.Message{Value: payload})

	require.NoError(t, err)
}

func TestEventHandler_Handle_UnsupportedVersion_ReturnsNil(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)
	videoMock := service_mocks.NewVideoMock(mc)

	h := newTestEventHandler(t, mc, videoMock)

	payload := []byte(
		`{"event_type":"ProcessingStarted","video_id":"` + uuid.New().String() + `","version":99}`,
	)

	err := h.Handle(t.Context(), kafka.Message{Value: payload})

	require.NoError(t, err)
}
