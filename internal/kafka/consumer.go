package kafka

import (
	"context"
	"fmt"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	defaultBackoffInitial = time.Second
	defaultBackoffMax     = 30 * time.Second
	backoffMultiplier     = 2
)

// Fetcher — минимальный набор операций чтения и ручного коммита сообщений, необходимый
// ReaderConsumer. Реализуется *kafka-go.Reader; выделен отдельно, чтобы подменять сеть
// фейком в юнит-тестах логики повторов.
type Fetcher interface {
	FetchMessage(ctx context.Context) (segmentio.Message, error)
	CommitMessages(ctx context.Context, msgs ...segmentio.Message) error
	Close() error
}

// ConsumerOption настраивает ReaderConsumer при создании.
type ConsumerOption func(*ReaderConsumer)

// WithBackoff задаёт начальную и максимальную паузу перед повтором необработанного сообщения.
// Используется в тестах, чтобы не ждать реальных секунд.
func WithBackoff(initial, maxDuration time.Duration) ConsumerOption {
	return func(c *ReaderConsumer) {
		c.backoffInitial = initial
		c.backoffMax = maxDuration
	}
}

// ReaderConsumer читает сообщения одного топика в составе consumer group через kafka-go Reader.
//
// Модель параллелизма — по одному Reader'у на процесс: обработка строго последовательна,
// коммит offset'а происходит только после успешного handle.
type ReaderConsumer struct {
	reader         Fetcher
	backoffInitial time.Duration
	backoffMax     time.Duration
}

// NewConsumer создаёт консьюмера топика topic в составе consumer group groupID.
//
// CommitInterval: 0 отключает автокоммит kafka-go — offset коммитится вручную через
// CommitMessages только после успешной обработки сообщения.
func NewConsumer(brokers []string, groupID, topic string, opts ...ConsumerOption) *ReaderConsumer {
	reader := segmentio.NewReader(segmentio.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		CommitInterval: 0,
	})

	return newReaderConsumer(reader, opts...)
}

// NewConsumerWithFetcher создаёт консьюмера поверх произвольной реализации Fetcher.
// Используется в юнит-тестах для подмены сети.
func NewConsumerWithFetcher(f Fetcher, opts ...ConsumerOption) *ReaderConsumer {
	return newReaderConsumer(f, opts...)
}

func newReaderConsumer(f Fetcher, opts ...ConsumerOption) *ReaderConsumer {
	c := &ReaderConsumer{
		reader:         f,
		backoffInitial: defaultBackoffInitial,
		backoffMax:     defaultBackoffMax,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Run блокируется до отмены ctx, читая сообщения по одному и передавая их в handle.
// При ошибке handle сообщение не коммитится: обработка повторяется для того же сообщения
// после паузы (экспоненциальный backoff от backoffInitial до backoffMax).
func (c *ReaderConsumer) Run(ctx context.Context, handle func(ctx context.Context, msg Message) error) error {
	for {
		raw, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return c.fetchError(ctx, err)
		}

		c.handleWithRetry(ctx, raw, handle)
	}
}

// fetchError отличает штатное завершение по отменённому контексту (graceful shutdown,
// ошибка не возвращается) от прочих ошибок чтения, которые прерывают Run.
func (c *ReaderConsumer) fetchError(ctx context.Context, fetchErr error) error {
	if ctx.Err() != nil {
		return nil //nolint:nilerr // штатное завершение Run по отменённому ctx — не ошибка
	}

	return fmt.Errorf("fetch message: %w", fetchErr)
}

// handleWithRetry вызывает handle для одного и того же сообщения raw до успеха или отмены ctx.
func (c *ReaderConsumer) handleWithRetry(
	ctx context.Context,
	raw segmentio.Message,
	handle func(ctx context.Context, msg Message) error,
) {
	backoff := c.backoffInitial
	msg := toMessage(raw)

	for {
		if err := handle(ctx, msg); err != nil {
			zap.L().Error("failed to handle kafka message, will retry",
				zap.String("topic", raw.Topic),
				zap.Int64("offset", raw.Offset),
				zap.Error(err),
			)

			if !c.wait(ctx, &backoff) {
				return
			}

			continue
		}

		if commitErr := c.reader.CommitMessages(ctx, raw); commitErr != nil {
			zap.L().Error("failed to commit kafka message",
				zap.String("topic", raw.Topic),
				zap.Int64("offset", raw.Offset),
				zap.Error(commitErr),
			)
		}

		return
	}
}

// wait приостанавливает обработку на backoff (увеличивая его вдвое, но не выше backoffMax)
// либо прерывается по отмене ctx. Возвращает false, если ожидание было прервано отменой ctx.
func (c *ReaderConsumer) wait(ctx context.Context, backoff *time.Duration) bool {
	timer := time.NewTimer(*backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		*backoff = min(*backoff*backoffMultiplier, c.backoffMax)
		return true
	}
}

// Close закрывает читателя Kafka.
func (c *ReaderConsumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("close kafka reader: %w", err)
	}

	return nil
}

func toMessage(raw segmentio.Message) Message {
	headers := make(map[string]string, len(raw.Headers))
	for _, h := range raw.Headers {
		headers[h.Key] = string(h.Value)
	}

	return Message{
		Topic:     raw.Topic,
		Partition: raw.Partition,
		Offset:    raw.Offset,
		Key:       raw.Key,
		Value:     raw.Value,
		Headers:   headers,
	}
}
