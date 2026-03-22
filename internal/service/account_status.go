package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"go.uber.org/zap"
)

type AccountStatusService struct {
	repo repository.AccountStatus
	srv  *Service
}

func NewAccountStatusService(repo repository.AccountStatus, srv *Service) *AccountStatusService {
	return &AccountStatusService{repo: repo, srv: srv}
}

func (s *AccountStatusService) Issue(
	ctx context.Context,
	userID string,
	status domain.BitPosition,
) (domain.AccountStatus, error) {
	users, err := s.repo.SelectByUsersID(ctx, userID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountStatus{}, err
	}
	if users == nil {
		return domain.AccountStatus{}, ErrNotFound
	}

	value := domain.SetBitsUpTo(domain.DefaultBitmap, status)

	accountStatus, err := s.repo.Upsert(ctx, userID, users[0].AccountID, value)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AccountStatus{}, err
	}

	return accountStatus, nil
}

func (s *AccountStatusService) GetByUsersID(ctx context.Context, usersID ...string) ([]domain.AccountStatus, error) {
	accountStatuses, err := s.repo.SelectByUsersID(ctx, usersID...)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return accountStatuses, nil
}
