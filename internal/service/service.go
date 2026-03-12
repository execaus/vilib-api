package service

import "vilib-api/internal/repository"

type Service struct {
	repo repository.Transactable
}

func NewService(r *repository.TransactionalRepository) *Service {
	s := &Service{
		repo: r,
	}

	return s
}
