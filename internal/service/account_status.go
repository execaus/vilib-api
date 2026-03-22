package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
)

type AccountStatusService struct {
	repo *repository.Repository
}

func NewAccountStatusService(repo *repository.Repository) *AccountStatusService {
	return &AccountStatusService{repo: repo}
}

func (s *AccountStatusService) Issue(
	ctx context.Context,
	userID, accountID string,
	status domain.BitPosition,
) (domain.BitmapValue, error) {
	value := domain.SetBitsUpTo(domain.DefaultBitmap, status)

	err := s.repo.AccountStatus.Upsert(ctx, userID, accountID, value)
	if err != nil {
		zap.L().Error(err.Error())
		return 0, err
	}

	return value, nil
}
