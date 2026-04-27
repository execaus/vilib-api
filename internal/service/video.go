package service

import (
	"context"
	"errors"
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
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionVideoWatch); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionVideoWatch); err != nil {
			return "", ErrForbidden
		}
	}

	// Получение видео по ID
	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	// Проверка, что видео принадлежит указанной группе
	if video.GroupID != groupID {
		zap.L().Error("video does not belong to the specified group")
		return "", ErrForbidden
	}

	// Получение ассетов видео
	assets, err := s.srv.VideoAsset.Get(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	// Определение, какой ассет использовать
	var (
		bucketName domain.VideoBucket
		assetID    uuid.UUID
	)

	if isPreferOriginal {
		// Всегда возвращаем оригинал
		for _, asset := range assets {
			if asset.Tag == domain.VideoAssetTagOriginal {
				assetID = asset.FileID
				bucketName = domain.VideoBucketOriginal
				break
			}
		}
	} else {
		// Пробуем сначала сжатую версию, если нет — оригинал
		hasCompressed := false
		for _, asset := range assets {
			if asset.Tag == domain.VideoAssetTagCompressed {
				assetID = asset.FileID
				bucketName = domain.VideoBucketCompressed
				hasCompressed = true
				break
			}
		}
		if !hasCompressed {
			for _, asset := range assets {
				if asset.Tag == domain.VideoAssetTagOriginal {
					assetID = asset.FileID
					bucketName = domain.VideoBucketOriginal
					break
				}
			}
		}
	}

	// Получение URL для стриминга видео
	preflightURL, err := s.s3.GetPreflightURL(ctx, bucketName, assetID, domain.VideoStreamURLTTL)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return preflightURL, nil
}

func (s *VideoService) GetPreflightUploadURL(
	ctx context.Context,
	accountID, groupID, userID uuid.UUID,
) (domain.PreflightURL, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, userID, domain.AccountPermissionManageVideo); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, userID, domain.GroupPermissionManageVideo); err != nil {
			return "", ErrForbidden
		}
	}

	// Создание записи о видео в статусе загрузки
	video, err := s.repo.Insert(ctx, domain.DefaultVideoName, groupID, userID, domain.VideoStatusUploading)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	// Получение URL для загрузки видео
	url, err := s.s3.GetPreflightUploadURL(ctx, domain.VideoBucketOriginal, video.ID, domain.VideoUploadURLTTL)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return url, nil
}

func (s *VideoService) Update(
	ctx context.Context,
	videoID uuid.UUID,
	initiatorID *uuid.UUID,
	status *domain.VideoStatus,
) (domain.Video, error) {
	// Получение видео по ID
	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			zap.L().Error(err.Error())
			return domain.Video{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	// Проверка прав доступа (только если передан инициатор — не для Kafka)
	if initiatorID != nil {
		if err := s.isCheckGroupAction(ctx, video.GroupID, *initiatorID, domain.GroupPermissionManageVideo); err != nil {
			zap.L().Error(err.Error())
			return domain.Video{}, err
		}
	}

	// Обновление статуса видео
	updatedVideo, err := s.repo.Update(ctx, videoID, status)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	return updatedVideo, nil
}

func (s *VideoService) GetAll(
	ctx context.Context,
	accountID, groupID, initiatorID uuid.UUID,
) ([]domain.Video, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionVideoWatch); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionVideoWatch); err != nil {
			return nil, ErrForbidden
		}
	}

	// Получение списка видео группы
	videos, err := s.repo.SelectByGroupID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return videos, nil
}

func (s *VideoService) Rename(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
	name string,
) (domain.Video, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageVideo); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionManageVideo); err != nil {
			return domain.Video{}, ErrForbidden
		}
	}

	// Переименование видео
	video, err := s.repo.UpdateName(ctx, videoID, name)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	return video, nil
}

func (s *VideoService) Delete(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
) error {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, initiatorID, domain.AccountPermissionManageVideo); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionManageVideo); err != nil {
			return ErrForbidden
		}
	}

	// Удаление видео
	if err := s.repo.Delete(ctx, videoID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

func (s *VideoService) isCheckGroupMember(
	ctx context.Context,
	groupID, userID uuid.UUID,
) error {
	// Проверка, является ли пользователь участником группы
	_, err := s.srv.GroupMember.GetByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		return ErrForbidden
	}

	return nil
}

func (s *VideoService) isCheckGroupAction(
	ctx context.Context,
	groupID, userID uuid.UUID,
	action domain.PermissionFlag,
) error {
	// Получение роли пользователя в группе
	member, err := s.srv.GroupMember.GetByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		// Если пользователь не состоит в группе — запрещено
		return ErrForbidden
	}

	// Получение group role
	roles, err := s.srv.GroupRole.GetByID(ctx, member.RoleID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// Проверка: является ли владельцем группы
	if domain.HasBit(roles[0].PermissionMask, domain.GroupPermissionOwner) {
		return nil
	}

	// Проверка наличия запрашиваемого разрешения
	if domain.HasBit(roles[0].PermissionMask, action) {
		return nil
	}

	return ErrForbidden
}
