package kafka_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"vilib-api/internal/kafka"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

// fakeFetcher — фейковая реализация kafka.Fetcher без сети: очередь сообщений в памяти
// и подсчёт коммитов/повторных вызовов.
type fakeFetcher struct {
	mu          sync.Mutex
	messages    []segmentio.Message
	fetchCalls  int
	committed   []segmentio.Message
	fetchErr    error
	closeCalled bool
}

func (f *fakeFetcher) FetchMessage(ctx context.Context) (segmentio.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.fetchCalls++

	if f.fetchErr != nil {
		return segmentio.Message{}, f.fetchErr
	}

	if len(f.messages) == 0 {
		<-ctx.Done()
		return segmentio.Message{}, ctx.Err()
	}

	msg := f.messages[0]
	f.messages = f.messages[1:]

	return msg, nil
}

func (f *fakeFetcher) CommitMessages(_ context.Context, msgs ...segmentio.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.committed = append(f.committed, msgs...)

	return nil
}

func (f *fakeFetcher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closeCalled = true

	return nil
}

func (f *fakeFetcher) fetchCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.fetchCalls
}

func (f *fakeFetcher) committedOffsets() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	offsets := make([]int64, len(f.committed))
	for i, m := range f.committed {
		offsets[i] = m.Offset
	}

	return offsets
}

// TestReaderConsumer_Run_CommitsAfterSuccessfulHandle проверяет, что успешно обработанное
// сообщение коммитится и консьюмер переходит к следующему.
func TestReaderConsumer_Run_CommitsAfterSuccessfulHandle(t *testing.T) {
	t.Parallel()

	fetcher := &fakeFetcher{
		messages: []segmentio.Message{
			{Topic: "video.original-uploaded", Offset: 1, Value: []byte("first")},
			{Topic: "video.original-uploaded", Offset: 2, Value: []byte("second")},
		},
	}
	consumer := kafka.NewConsumerWithFetcher(fetcher, kafka.WithBackoff(time.Millisecond, 5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var handled atomic.Int32
	done := make(chan error, 1)

	go func() {
		done <- consumer.Run(ctx, func(_ context.Context, _ kafka.Message) error {
			if handled.Add(1) == 2 {
				cancel()
			}

			return nil
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("consumer.Run did not stop after context cancellation")
	}

	require.Equal(t, []int64{1, 2}, fetcher.committedOffsets())
}

// TestReaderConsumer_Run_RetriesSameMessageOnHandleError проверяет, что при ошибке handle
// сообщение не коммитится и переобрабатывается то же сообщение (без повторного FetchMessage),
// а после успеха — коммитится.
func TestReaderConsumer_Run_RetriesSameMessageOnHandleError(t *testing.T) {
	t.Parallel()

	fetcher := &fakeFetcher{
		messages: []segmentio.Message{
			{Topic: "video.original-uploaded", Offset: 42, Value: []byte("payload")},
		},
	}
	consumer := kafka.NewConsumerWithFetcher(fetcher, kafka.WithBackoff(time.Millisecond, 5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	done := make(chan error, 1)

	go func() {
		done <- consumer.Run(ctx, func(_ context.Context, _ kafka.Message) error {
			if attempts.Add(1) < 3 {
				return errors.New("temporary failure")
			}

			cancel()

			return nil
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("consumer.Run did not stop after context cancellation")
	}

	require.Equal(t, int32(3), attempts.Load())
	// Один вызов на исходное получение сообщения и один — уже после коммита и отмены ctx
	// (блокируется на ctx.Done() у фейка и сразу возвращается). Повторные попытки обработки
	// не порождают новых FetchMessage — это и есть проверяемое поведение.
	require.Equal(t, 2, fetcher.fetchCallCount())
	require.Equal(t, []int64{42}, fetcher.committedOffsets())
}

// TestReaderConsumer_Run_StopsWithoutCommitOnShutdownDuringRetry проверяет, что при отмене
// ctx во время паузы backoff сообщение остаётся некоммиченным.
func TestReaderConsumer_Run_StopsWithoutCommitOnShutdownDuringRetry(t *testing.T) {
	t.Parallel()

	fetcher := &fakeFetcher{
		messages: []segmentio.Message{
			{Topic: "video.original-uploaded", Offset: 7, Value: []byte("payload")},
		},
	}
	consumer := kafka.NewConsumerWithFetcher(fetcher, kafka.WithBackoff(50*time.Millisecond, time.Second))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- consumer.Run(ctx, func(_ context.Context, _ kafka.Message) error {
			return errors.New("permanent-looking failure")
		})
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("consumer.Run did not stop after context cancellation")
	}

	require.Empty(t, fetcher.committedOffsets())
}

// TestReaderConsumer_Run_CompletesInFlightHandleAndCommitsAfterCtxCancelledDuringHandle
// проверяет семантику graceful shutdown (Э1-Т26): если ctx консьюмера отменяется, пока
// текущее сообщение уже обрабатывается, handle не прерывается — получает контекст без
// отмены — и, завершившись успехом, сообщение коммитится.
func TestReaderConsumer_Run_CompletesInFlightHandleAndCommitsAfterCtxCancelledDuringHandle(t *testing.T) {
	t.Parallel()

	fetcher := &fakeFetcher{
		messages: []segmentio.Message{
			{Topic: "video.processing-events", Offset: 1, Value: []byte("payload")},
		},
	}
	consumer := kafka.NewConsumerWithFetcher(fetcher, kafka.WithBackoff(time.Millisecond, 5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())

	handleStarted := make(chan struct{})
	proceed := make(chan struct{})

	var handleSawCancel atomic.Bool

	done := make(chan error, 1)
	go func() {
		done <- consumer.Run(ctx, func(handleCtx context.Context, _ kafka.Message) error {
			close(handleStarted)
			<-proceed

			if handleCtx.Err() != nil {
				handleSawCancel.Store(true)
			}

			return nil
		})
	}()

	<-handleStarted
	cancel()
	// Даём отменённому ctx время дойти до горутины до того, как handle продолжит работу и
	// проверит handleCtx.Err().
	time.Sleep(20 * time.Millisecond)
	close(proceed)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("consumer.Run did not stop after context cancellation")
	}

	require.False(t, handleSawCancel.Load(), "handle must not observe cancellation of the outer ctx")
	require.Equal(t, []int64{1}, fetcher.committedOffsets())
}

// TestReaderConsumer_Run_ReturnsErrorOnFetchFailure проверяет, что ошибка FetchMessage,
// не связанная с отменой ctx, возвращается из Run.
func TestReaderConsumer_Run_ReturnsErrorOnFetchFailure(t *testing.T) {
	t.Parallel()

	fetcher := &fakeFetcher{fetchErr: errors.New("broker unavailable")}
	consumer := kafka.NewConsumerWithFetcher(fetcher)

	err := consumer.Run(context.Background(), func(_ context.Context, _ kafka.Message) error {
		return nil
	})

	require.Error(t, err)
}

// TestReaderConsumer_Close закрывает нижележащий Fetcher.
func TestReaderConsumer_Close(t *testing.T) {
	t.Parallel()

	fetcher := &fakeFetcher{}
	consumer := kafka.NewConsumerWithFetcher(fetcher)

	require.NoError(t, consumer.Close())
	require.True(t, fetcher.closeCalled)
}
