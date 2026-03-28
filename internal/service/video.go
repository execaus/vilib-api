package service

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/s3"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type VideoService struct {
	s3   s3.S3
	repo repository.Video
	srv  *Service
}

func NewVideoService(s3 s3.S3, repo repository.Video, srv *Service) *VideoService {
	return &VideoService{s3: s3, repo: repo, srv: srv}
}

func (s *VideoService) Get(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
	isPreferOriginal bool,
) (domain.PreflightURL, error) {

	s.repo.Select(ctx, videoID)

	s.srv.VideoAsset.Get(ctx, videoID)

	var (
		bucketName domain.VideoBucket
		assetID    uuid.UUID
	)
	preflightURL, _ := s.s3.GetPreflightURL(ctx, bucketName, assetID, domain.VideoStreamURLTTL)

	return preflightURL, nil
}

func (s *VideoService) GetPreflightUploadURL(
	ctx context.Context,
	accountID, groupID, userID uuid.UUID,
) (domain.PreflightURL, error) {
	video, _ := s.repo.Insert(ctx, domain.DefaultVideoName, groupID, userID, domain.VideoStatusUploading)

	url, _ := s.s3.GetPreflightUploadURL(ctx, domain.VideoBucketOriginal, video.ID, domain.VideoUploadURLTTL)

	return url, nil
}

func (s *VideoService) Update(
	ctx context.Context,
	videoID uuid.UUID,
	status *domain.VideoStatus,
) (domain.Video, error) {
	video, err := s.repo.Update(ctx, videoID, status)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	return video, nil
}
