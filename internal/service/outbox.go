package service

import (
	"context"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
)

type OutboxService struct {
	repo repository.Outbox
	srv  *Service
}

func NewOutboxService(repo repository.Outbox, srv *Service) *OutboxService {
	return &OutboxService{repo: repo, srv: srv}
}

// Publish кладёт событие в очередь публикации Kafka внутри текущей транзакции саги —
// прокси на repository.Outbox.Insert (transactional outbox, §7.1 эпика).
func (s *OutboxService) Publish(ctx context.Context, topic, key string, payload []byte) error {
	if err := s.repo.Insert(ctx, topic, key, payload); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
