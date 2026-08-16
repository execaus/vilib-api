// Package kafka содержит тонкие обёртки над kafka-go: продюсер и однопартиционный
// консьюмер с ручным коммитом offset'ов.
package kafka

import "context"

//go:generate minimock -i Producer -o ./kafka_mocks/producer_mock.go
//go:generate minimock -i Consumer -o ./kafka_mocks/consumer_mock.go

// Message — сообщение Kafka, полученное консьюмером.
type Message struct {
	// Topic — топик, из которого получено сообщение.
	Topic string
	// Partition — партиция сообщения.
	Partition int
	// Offset — смещение сообщения в партиции.
	Offset int64
	// Key — ключ сообщения.
	Key []byte
	// Value — тело сообщения.
	Value []byte
	// Headers — заголовки сообщения.
	Headers map[string]string
}

// Producer публикует сообщения в топик Kafka.
type Producer interface {
	// Publish публикует сообщение value с ключом key и заголовками headers в топик topic.
	Publish(ctx context.Context, topic, key string, value []byte, headers map[string]string) error
}

// Consumer читает сообщения одного топика в составе consumer group и передаёт их в handle
// строго последовательно.
type Consumer interface {
	// Run блокируется до отмены ctx. Сообщение коммитится только при успешной обработке
	// (handle вернул nil); при ошибке сообщение не коммитится, обработка того же сообщения
	// повторяется после паузы с экспоненциальным backoff'ом.
	Run(ctx context.Context, handle func(ctx context.Context, msg Message) error) error
	// Close закрывает читателя Kafka.
	Close() error
}
