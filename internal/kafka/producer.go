package kafka

import (
	"context"
	"fmt"

	segmentio "github.com/segmentio/kafka-go"
)

// WriterProducer публикует сообщения через kafka-go Writer.
type WriterProducer struct {
	writer *segmentio.Writer
}

// NewProducer создаёт продюсера поверх списка брокеров brokers.
//
// Балансировка по ключу (Hash) сохраняет порядок сообщений одного video_id в одной партиции;
// RequireAll ждёт подтверждения от всех реплик, Async=false делает Publish синхронным.
func NewProducer(brokers []string) *WriterProducer {
	return &WriterProducer{
		writer: &segmentio.Writer{
			Addr:         segmentio.TCP(brokers...),
			Balancer:     &segmentio.Hash{},
			RequiredAcks: segmentio.RequireAll,
			Async:        false,
		},
	}
}

// Publish публикует сообщение value с ключом key и заголовками headers в топик topic.
func (p *WriterProducer) Publish(
	ctx context.Context,
	topic, key string,
	value []byte,
	headers map[string]string,
) error {
	msg := segmentio.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   value,
		Headers: toKafkaHeaders(headers),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publish message to topic %s: %w", topic, err)
	}

	return nil
}

// Close закрывает продюсера, дожидаясь отправки буферизованных сообщений.
func (p *WriterProducer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}

	return nil
}

func toKafkaHeaders(headers map[string]string) []segmentio.Header {
	if len(headers) == 0 {
		return nil
	}

	result := make([]segmentio.Header, 0, len(headers))
	for k, v := range headers {
		result = append(result, segmentio.Header{Key: k, Value: []byte(v)})
	}

	return result
}
