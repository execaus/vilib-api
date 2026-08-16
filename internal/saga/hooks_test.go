package saga_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"vilib-api/internal/saga"
	"vilib-api/internal/saga/saga_mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/stretchr/testify/require"
)

// newRunnerWithTx собирает Runner с моком Transactable, у которого WithTx отдаёт tx
// с настраиваемым результатом Commit (commitErr) и всегда успешным Rollback.
func newRunnerWithTx(t *testing.T, commitErr error) saga.Runner[struct{}] {
	t.Helper()

	mc := minimock.NewController(t)
	tx := saga_mocks.NewBobTransactionMock(mc)
	tx.CommitMock.Return(commitErr)
	tx.CommitMock.Optional()
	tx.RollbackMock.Return(nil)
	tx.RollbackMock.Optional()

	repo := saga_mocks.NewTransactableMock(mc)
	repo.WithTxMock.Return(tx, nil)

	return saga.NewSagaRunner(struct{}{}, repo)
}

// TestAfterCommit_RunsHookAfterCommit проверяет, что хук выполняется после успешного Commit.
func TestAfterCommit_RunsHookAfterCommit(t *testing.T) {
	t.Parallel()

	runner := newRunnerWithTx(t, nil)

	var called bool
	err := runner.Run(t.Context(), func(ctx context.Context, _ struct{}) error {
		saga.AfterCommit(ctx, func(context.Context) {
			called = true
		})
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
}

// TestAfterCommit_NotCalledOnCallbackError проверяет, что хук не вызывается, если колбэк
// саги вернул ошибку (транзакция откатывается).
func TestAfterCommit_NotCalledOnCallbackError(t *testing.T) {
	t.Parallel()

	runner := newRunnerWithTx(t, nil)

	var called bool
	callbackErr := errors.New("callback failed")

	err := runner.Run(t.Context(), func(ctx context.Context, _ struct{}) error {
		saga.AfterCommit(ctx, func(context.Context) {
			called = true
		})
		return callbackErr
	})

	require.ErrorIs(t, err, callbackErr)
	require.False(t, called)
}

// TestAfterCommit_NotCalledOnCommitError проверяет, что хук не вызывается, если Commit
// транзакции завершился ошибкой.
func TestAfterCommit_NotCalledOnCommitError(t *testing.T) {
	t.Parallel()

	commitErr := errors.New("commit failed")
	runner := newRunnerWithTx(t, commitErr)

	var called bool
	err := runner.Run(t.Context(), func(ctx context.Context, _ struct{}) error {
		saga.AfterCommit(ctx, func(context.Context) {
			called = true
		})
		return nil
	})

	require.ErrorIs(t, err, commitErr)
	require.False(t, called)
}

// TestAfterCommit_RunsHooksInOrder проверяет, что несколько хуков выполняются в порядке
// регистрации.
func TestAfterCommit_RunsHooksInOrder(t *testing.T) {
	t.Parallel()

	runner := newRunnerWithTx(t, nil)

	var order []int
	err := runner.Run(t.Context(), func(ctx context.Context, _ struct{}) error {
		saga.AfterCommit(ctx, func(context.Context) { order = append(order, 1) })
		saga.AfterCommit(ctx, func(context.Context) { order = append(order, 2) })
		saga.AfterCommit(ctx, func(context.Context) { order = append(order, 3) })
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, order)
}

// TestAfterCommit_PanicInHookDoesNotFailRun проверяет, что паника в одном хуке не роняет
// Run и не мешает выполнению последующих хуков.
func TestAfterCommit_PanicInHookDoesNotFailRun(t *testing.T) {
	t.Parallel()

	runner := newRunnerWithTx(t, nil)

	var afterPanicCalled bool
	err := runner.Run(t.Context(), func(ctx context.Context, _ struct{}) error {
		saga.AfterCommit(ctx, func(context.Context) {
			panic("boom")
		})
		saga.AfterCommit(ctx, func(context.Context) {
			afterPanicCalled = true
		})
		return nil
	})

	require.NoError(t, err)
	require.True(t, afterPanicCalled)
}

// TestAfterCommit_OutsideSagaDoesNotPanic проверяет, что вызов AfterCommit вне saga.Run
// не паникует и молча отбрасывает хук.
func TestAfterCommit_OutsideSagaDoesNotPanic(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		saga.AfterCommit(t.Context(), func(context.Context) {
			t.Fatal("hook must not be called outside of saga.Run")
		})
	})
}

// TestAfterCommit_HookReceivesCtxWithoutTx проверяет, что хук получает ctx без ключа
// транзакции саги — репозитории через ExecutorProvider не должны попадать в закрытую
// транзакцию.
func TestAfterCommit_HookReceivesCtxWithoutTx(t *testing.T) {
	t.Parallel()

	runner := newRunnerWithTx(t, nil)

	var mu sync.Mutex
	var sagaTxValue any

	err := runner.Run(t.Context(), func(ctx context.Context, _ struct{}) error {
		saga.AfterCommit(ctx, func(hookCtx context.Context) {
			mu.Lock()
			defer mu.Unlock()
			sagaTxValue = hookCtx.Value(saga.CtxKey)
		})
		return nil
	})

	require.NoError(t, err)
	require.Nil(t, sagaTxValue)
}
