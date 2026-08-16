package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type VideoAssetService struct {
	repo repository.VideoAsset
	srv  *Service
}

func (s *VideoAssetService) Get(ctx context.Context, videoID uuid.UUID) ([]domain.VideoAsset, error) {
	// Получение ассетов видео
	assets, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assets, nil
}

func NewVideoAssetService(repo repository.VideoAsset, srv *Service) *VideoAssetService {
	return &VideoAssetService{repo: repo, srv: srv}
}

func (s *VideoAssetService) Create(
	ctx context.Context,
	videoID uuid.UUID,
	kind domain.VideoAssetKind,
	profile domain.VideoProfile,
	bucket, key, contentType string,
	sizeBytes int64,
) (domain.VideoAsset, error) {
	// Создание ассета видео
	asset, err := s.repo.Insert(ctx, videoID, kind, profile, bucket, key, contentType, sizeBytes)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAsset{}, err
	}

	return asset, nil
}
