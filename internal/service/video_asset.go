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

// SelectByVideoIDs выбирает ассеты сразу нескольких видео вместе с данными связанных файлов
// (Э1-Т20, список видео группы). Пустой список идентификаторов не порождает запроса к БД.
func (s *VideoAssetService) SelectByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) ([]domain.VideoAsset, error) {
	assets, err := s.repo.SelectByVideoIDs(ctx, videoIDs)
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

// DeleteByVideoAndKinds удаляет ассеты видео указанных видов вместе со связанными файлами —
// идемпотентная перерегистрация результатов обработки при повторном ProcessingCompleted (Э1-Т14).
func (s *VideoAssetService) DeleteByVideoAndKinds(
	ctx context.Context,
	videoID uuid.UUID,
	kinds []domain.VideoAssetKind,
) error {
	if err := s.repo.DeleteByVideoAndKinds(ctx, videoID, kinds); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
