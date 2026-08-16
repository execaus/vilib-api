package watchdog_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"
	"vilib-api/internal/watchdog"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testTimeout = 2 * time.Second

// newTestRunner собирает saga.Runner поверх мока Transactable: WithTx всегда отдаёт один и тот
// же мок транзакции, Commit/Rollback можно вызывать сколько угодно раз (по аналогии с тестами
// релея outbox, internal/outbox/relay_test.go).
func newTestRunner(t *testing.T, svc *service.Service) saga.Runner[*service.Service] {
	t.Helper()

	mc := minimock.NewController(t)

	tx := saga_mocks.NewBobTransactionMock(mc)
	tx.CommitMock.Return(nil)
	tx.CommitMock.Optional()
	tx.RollbackMock.Return(nil)
	tx.RollbackMock.Optional()

	repo := saga_mocks.NewTransactableMock(mc)
	repo.WithTxMock.Return(tx, nil)

	return saga.NewSagaRunner(svc, repo)
}

// runUntil запускает wd.Run в фоне и блокируется до срабатывания done либо таймаута, после
// чего отменяет контекст и дожидается завершения Run.
func runUntil(t *testing.T, wd *watchdog.Watchdog, done <-chan struct{}) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- wd.Run(ctx)
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
		t.Fatal("watchdog.Run did not stop after context cancellation")
	}
}

// TestWatchdog_Run_TicksImmediatelyAtStart проверяет, что первый тик выполняется сразу при
// старте, не дожидаясь первого срабатывания тикера (§8 дизайна эпика).
func TestWatchdog_Run_TicksImmediatelyAtStart(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	done := make(chan struct{})
	var calls atomic.Int32
	videoMock := service_mocks.NewVideoMock(mc)
	videoMock.FailTimedOutMock.Set(func(context.Context, time.Time) (domain.TimedOutReport, error) {
		if calls.Add(1) == 1 {
			close(done)
		}
		return domain.TimedOutReport{}, nil
	})

	svc := &service.Service{Video: videoMock}
	runner := newTestRunner(t, svc)

	// Интервал заведомо больше времени ожидания в тесте — если бы первый тик ждал тикер,
	// done не закрылся бы до истечения testTimeout.
	wd := watchdog.New(runner, config.VideoConfig{WatchdogInterval: time.Hour}, zap.NewNop())

	runUntil(t, wd, done)

	require.Equal(t, int32(1), calls.Load())
}

// TestWatchdog_Run_ContinuesAfterTickError проверяет, что ошибка сервиса на одном тике не
// останавливает watchdog — он продолжает тикать дальше (§8 дизайна эпика: единичный сбой не
// должен отключать страховочный процесс).
func TestWatchdog_Run_ContinuesAfterTickError(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	done := make(chan struct{})
	var calls atomic.Int32
	errTick := errors.New("db unavailable")

	videoMock := service_mocks.NewVideoMock(mc)
	videoMock.FailTimedOutMock.Set(func(context.Context, time.Time) (domain.TimedOutReport, error) {
		n := calls.Add(1)
		if n == 1 {
			return domain.TimedOutReport{}, errTick
		}
		if n == 3 {
			close(done)
		}
		return domain.TimedOutReport{}, nil
	})

	svc := &service.Service{Video: videoMock}
	runner := newTestRunner(t, svc)

	wd := watchdog.New(runner, config.VideoConfig{WatchdogInterval: 5 * time.Millisecond}, zap.NewNop())

	runUntil(t, wd, done)

	require.GreaterOrEqual(t, calls.Load(), int32(3))
}

// TestWatchdog_Run_StopsOnContextCancellation проверяет, что Run завершается без ошибки после
// отмены контекста и не выполняет тиков после этого.
func TestWatchdog_Run_StopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	mc := minimock.NewController(t)

	var calls atomic.Int32
	videoMock := service_mocks.NewVideoMock(mc)
	videoMock.FailTimedOutMock.Set(func(context.Context, time.Time) (domain.TimedOutReport, error) {
		calls.Add(1)
		return domain.TimedOutReport{}, nil
	})

	svc := &service.Service{Video: videoMock}
	runner := newTestRunner(t, svc)

	wd := watchdog.New(runner, config.VideoConfig{WatchdogInterval: time.Hour}, zap.NewNop())

	ctx, cancel := context.WithCancel(t.Context())

	runDone := make(chan error, 1)
	go func() {
		runDone <- wd.Run(ctx)
	}()

	// Дожидаемся немедленного первого тика перед отменой ctx.
	require.Eventually(t, func() bool { return calls.Load() == 1 }, testTimeout, time.Millisecond)

	cancel()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(testTimeout):
		t.Fatal("watchdog.Run did not stop after context cancellation")
	}
}
