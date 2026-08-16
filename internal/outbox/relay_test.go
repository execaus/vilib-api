package outbox_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/kafka/kafka_mocks"
	"vilib-api/internal/outbox"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testTimeout = 2 * time.Second

var errBrokerUnavailable = errors.New("broker unavailable")

// newTestRunner собирает saga.Runner поверх мока Transactable: WithTx всегда отдаёт один
// и тот же мок транзакции, Commit/Rollback можно вызывать сколько угодно раз.
func newTestRunner(t *testing.T) saga.Runner[*service.Service] {
	t.Helper()

	mc := minimock.NewController(t)

	tx := saga_mocks.NewBobTransactionMock(mc)
	tx.CommitMock.Return(nil)
	tx.CommitMock.Optional()
	tx.RollbackMock.Return(nil)
	tx.RollbackMock.Optional()

	repo := saga_mocks.NewTransactableMock(mc)
	repo.WithTxMock.Return(tx, nil)

	return saga.NewSagaRunner[*service.Service](nil, repo)
}

// runUntil запускает relay.Run в фоне и блокируется до срабатывания done либо таймаута,
// после чего отменяет контекст и дожидается завершения Run.
func runUntil(t *testing.T, r *outbox.Relay, done <-chan struct{}) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- r.Run(ctx)
	}()

	select {
	case <-done:
	case <-time.After(testTimeout):
		cancel()
		t.Fatal("condition was not met before timeout")
	}

	cancel()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(testTimeout):
		t.Fatal("relay.Run did not stop after context cancellation")
	}
}

func TestRelay_Run_PublishesAndDeletesBatch(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	events := []domain.OutboxEvent{
		{
			ID: 1, Topic: "video.original-uploaded", Key: "video-1",
			Payload: []byte(`{"event_type":"OriginalUploaded","version":1}`),
		},
		{ID: 2, Topic: "video.original-uploaded", Key: "video-2", Payload: []byte(`not-json`)},
	}

	repo := repository_mocks.NewOutboxMock(mc)

	var selectCalls atomic.Int32
	repo.SelectBatchForUpdateMock.Set(func(context.Context, int) ([]domain.OutboxEvent, error) {
		if selectCalls.Add(1) == 1 {
			return events, nil
		}
		return nil, nil
	})

	producer := kafka_mocks.NewProducerMock(mc)

	type publishCall struct {
		topic, key string
		headers    map[string]string
	}
	var mu sync.Mutex
	var publishes []publishCall

	producer.PublishMock.Set(func(_ context.Context, topic, key string, _ []byte, headers map[string]string) error {
		mu.Lock()
		defer mu.Unlock()
		publishes = append(publishes, publishCall{topic: topic, key: key, headers: headers})
		return nil
	})

	done := make(chan struct{})
	var deletedIDs []int64
	repo.DeleteByIDsMock.Set(func(_ context.Context, ids []int64) error {
		mu.Lock()
		deletedIDs = ids
		mu.Unlock()
		close(done)
		return nil
	})

	runner := newTestRunner(t)
	relay := outbox.NewRelay(runner, repo, producer, 5*time.Millisecond, 10, zap.NewNop())

	runUntil(t, relay, done)

	mu.Lock()
	defer mu.Unlock()

	require.Equal(t, []int64{1, 2}, deletedIDs)
	require.Len(t, publishes, 2)
	require.Equal(t, "video.original-uploaded", publishes[0].topic)
	require.Equal(t, "video-1", publishes[0].key)
	require.Equal(t, map[string]string{"event-type": "OriginalUploaded", "event-version": "1"}, publishes[0].headers)
	require.Equal(t, "video-2", publishes[1].key)
	require.Nil(t, publishes[1].headers)
}

func TestRelay_Run_PublishErrorKeepsRowsAndRollsBack(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	events := []domain.OutboxEvent{
		{ID: 1, Topic: "video.original-uploaded", Key: "video-1", Payload: []byte(`{}`)},
	}

	repo := repository_mocks.NewOutboxMock(mc)

	var selectCalls atomic.Int32
	repo.SelectBatchForUpdateMock.Set(func(context.Context, int) ([]domain.OutboxEvent, error) {
		if selectCalls.Add(1) == 1 {
			return events, nil
		}
		return nil, nil
	})

	var deleteCalls atomic.Int32
	repo.DeleteByIDsMock.Set(func(context.Context, []int64) error {
		deleteCalls.Add(1)
		return nil
	})
	repo.DeleteByIDsMock.Optional()

	producer := kafka_mocks.NewProducerMock(mc)
	producer.PublishMock.Return(errBrokerUnavailable)

	tx := saga_mocks.NewBobTransactionMock(mc)
	tx.CommitMock.Return(nil)
	tx.CommitMock.Optional()

	done := make(chan struct{})
	var rollbackCalls atomic.Int32
	tx.RollbackMock.Set(func(context.Context) error {
		if rollbackCalls.Add(1) == 1 {
			close(done)
		}
		return nil
	})

	repoTx := saga_mocks.NewTransactableMock(mc)
	repoTx.WithTxMock.Return(tx, nil)

	runner := saga.NewSagaRunner[*service.Service](nil, repoTx)
	relay := outbox.NewRelay(runner, repo, producer, 5*time.Millisecond, 10, zap.NewNop())

	runUntil(t, relay, done)

	require.Zero(t, tx.CommitAfterCounter())
	require.Zero(t, deleteCalls.Load())
}

func TestRelay_Run_EmptyBatchPublishesNothing(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	repo := repository_mocks.NewOutboxMock(mc)

	done := make(chan struct{})
	var selectCalls atomic.Int32
	repo.SelectBatchForUpdateMock.Set(func(context.Context, int) ([]domain.OutboxEvent, error) {
		if selectCalls.Add(1) == 3 {
			close(done)
		}
		return nil, nil
	})

	var publishCalls, deleteCalls atomic.Int32

	producer := kafka_mocks.NewProducerMock(mc)
	producer.PublishMock.Set(func(context.Context, string, string, []byte, map[string]string) error {
		publishCalls.Add(1)
		return nil
	})
	producer.PublishMock.Optional()

	repo.DeleteByIDsMock.Set(func(context.Context, []int64) error {
		deleteCalls.Add(1)
		return nil
	})
	repo.DeleteByIDsMock.Optional()

	runner := newTestRunner(t)
	relay := outbox.NewRelay(runner, repo, producer, 5*time.Millisecond, 10, zap.NewNop())

	runUntil(t, relay, done)

	require.Zero(t, publishCalls.Load())
	require.Zero(t, deleteCalls.Load())
}

func TestRelay_Run_FullBatchTriggersImmediateNextTick(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	repo := repository_mocks.NewOutboxMock(mc)

	const batchSize = 2

	firstBatch := []domain.OutboxEvent{
		{ID: 1, Topic: "t", Key: "k1", Payload: []byte(`{}`)},
		{ID: 2, Topic: "t", Key: "k2", Payload: []byte(`{}`)},
	}
	secondBatch := []domain.OutboxEvent{
		{ID: 3, Topic: "t", Key: "k3", Payload: []byte(`{}`)},
	}

	const interval = 300 * time.Millisecond

	done := make(chan struct{})
	var selectCalls atomic.Int32
	var mu sync.Mutex
	var firstCallAt, secondCallAt time.Time
	repo.SelectBatchForUpdateMock.Set(func(context.Context, int) ([]domain.OutboxEvent, error) {
		switch selectCalls.Add(1) {
		case 1:
			mu.Lock()
			firstCallAt = time.Now()
			mu.Unlock()
			return firstBatch, nil
		case 2:
			mu.Lock()
			secondCallAt = time.Now()
			mu.Unlock()
			close(done)
			return secondBatch, nil
		default:
			return nil, nil
		}
	})
	repo.DeleteByIDsMock.Return(nil)

	producer := kafka_mocks.NewProducerMock(mc)
	producer.PublishMock.Return(nil)

	runner := newTestRunner(t)
	relay := outbox.NewRelay(runner, repo, producer, interval, batchSize, zap.NewNop())

	runUntil(t, relay, done)

	mu.Lock()
	defer mu.Unlock()
	// Второй вызов SelectBatchForUpdate должен произойти сразу после первого (немедленный
	// повторный тик после полного батча), а не только спустя interval по таймеру.
	require.Less(t, secondCallAt.Sub(firstCallAt), interval/2)
}
