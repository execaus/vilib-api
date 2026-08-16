package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"
	"vilib-api/internal/s3"

	"github.com/google/uuid"
	"go.uber.org/zap"

	events "github.com/execaus/vilib-events"
)

// videoContentTypePrefix — обязательный префикс MIME-типа загружаемого видео (Э1-Т9, В-9 эпика).
const videoContentTypePrefix = "video/"

// videoInitialProcessingAttempt — номер попытки обработки, с которого видео впервые
// становится в очередь при подтверждении загрузки.
const videoInitialProcessingAttempt = 1

// VideoServiceConfig — часть конфигурации приложения, используемая сервисом видео.
type VideoServiceConfig struct {
	// Bucket — бакет S3, в котором хранятся объекты видео.
	Bucket string
	// Video — параметры обработки видео (лимиты, профили, таймауты).
	Video config.VideoConfig
	// TopicOriginalUploaded — топик Kafka для публикации события OriginalUploaded.
	TopicOriginalUploaded string
}

type VideoService struct {
	s3   s3.S3
	repo repository.Video
	srv  *Service
	cfg  VideoServiceConfig
}

func NewVideoService(s3 s3.S3, repo repository.Video, srv *Service, cfg VideoServiceConfig) *VideoService {
	return &VideoService{s3: s3, repo: repo, srv: srv, cfg: cfg}
}

// findAssetByKind ищет первый ассет указанного вида. Возвращает nil, если такого нет.
func findAssetByKind(assets []domain.VideoAsset, kind domain.VideoAssetKind) *domain.VideoAsset {
	for i := range assets {
		if assets[i].Kind == kind {
			return &assets[i]
		}
	}

	return nil
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

	// Определение, какой ассет использовать. Ассет hls_master — временная замена
	// «сжатой» версии до полноценной HLS-выдачи (В-7).
	var selected *domain.VideoAsset

	if isPreferOriginal {
		selected = findAssetByKind(assets, domain.VideoAssetKindOriginal)
	} else {
		selected = findAssetByKind(assets, domain.VideoAssetKindHLSMaster)
		if selected == nil {
			selected = findAssetByKind(assets, domain.VideoAssetKindOriginal)
		}
	}

	if selected == nil {
		zap.L().Error("no suitable video asset found")
		return "", ErrNotFound
	}

	// Получение URL для стриминга видео
	preflightURL, err := s.s3.PresignGetObject(ctx, selected.Bucket, selected.ObjectKey, domain.VideoStreamURLTTL)
	if err != nil {
		zap.L().Error(err.Error())
		return "", err
	}

	return preflightURL, nil
}

// CreateUpload проверяет права ManageVideo, валидирует content-type и размер файла,
// создаёт запись видео в статусе uploading и выдаёт преподписанный URL на PUT-загрузку
// оригинала в хранилище (Э1-Т7, §5 дизайна эпика).
func (s *VideoService) CreateUpload(
	ctx context.Context,
	accountID, groupID, userID uuid.UUID,
	name, contentType string,
	size int64,
) (domain.VideoUpload, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, userID, domain.AccountPermissionManageVideo); err != nil {
		if groupErr := s.isCheckGroupAction(ctx, groupID, userID, domain.GroupPermissionManageVideo); groupErr != nil {
			return domain.VideoUpload{}, ErrForbidden
		}
	}

	// Валидация MIME-типа и размера файла
	if !strings.HasPrefix(contentType, videoContentTypePrefix) {
		return domain.VideoUpload{}, NewValidationError("content_type must start with " + videoContentTypePrefix)
	}
	if size <= 0 || size > s.cfg.Video.MaxUploadSizeBytes {
		return domain.VideoUpload{}, NewValidationError("size_bytes must be between 1 and max upload size")
	}

	// Создание записи о видео в статусе загрузки
	video, err := s.repo.Insert(ctx, name, groupID, userID, domain.VideoStatusUploading)
	if err != nil {
		if errors.Is(dberrors.UserGroupVideoErrors.ErrUniqueUserGroupVideosUserGroupIdNameKey, err) {
			zap.L().Warn(err.Error())
			return domain.VideoUpload{}, NewConflictError("video name already exists")
		}
		zap.L().Error(err.Error())
		return domain.VideoUpload{}, err
	}

	// Получение URL для загрузки оригинала видео
	key := domain.VideoOriginalObjectKey(video.ID)
	url, err := s.s3.PresignPutObject(ctx, s.cfg.Bucket, key, contentType, size, domain.VideoUploadURLTTL)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoUpload{}, err
	}

	return domain.VideoUpload{
		VideoID:   video.ID,
		UploadURL: url,
		ExpiresAt: time.Now().Add(domain.VideoUploadURLTTL),
	}, nil
}

// CompleteUpload подтверждает загрузку оригинала видео: проверяет объект в хранилище,
// регистрирует ассет-оригинал, переводит видео в очередь на обработку и публикует событие
// OriginalUploaded через outbox. Повторный вызов для видео, уже поставленного в очередь,
// обрабатываемого или готового, идемпотентен и не имеет побочных эффектов (Э1-Т9…Т11).
func (s *VideoService) CompleteUpload(
	ctx context.Context,
	accountID, groupID, userID, videoID uuid.UUID,
) (domain.Video, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(ctx, accountID, userID, domain.AccountPermissionManageVideo); err != nil {
		if groupErr := s.isCheckGroupAction(ctx, groupID, userID, domain.GroupPermissionManageVideo); groupErr != nil {
			return domain.Video{}, ErrForbidden
		}
	}

	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	// Проверка, что видео принадлежит указанной группе
	if video.GroupID != groupID {
		zap.L().Error("video does not belong to the specified group")
		return domain.Video{}, ErrForbidden
	}

	switch video.Status {
	case domain.VideoStatusQueued, domain.VideoStatusCompressing, domain.VideoStatusReady:
		// Повторное подтверждение уже принятой загрузки — идемпотентный no-op.
		return *video, nil
	case domain.VideoStatusFailed:
		reason := "timeout"
		if video.FailureReason != nil {
			reason = *video.FailureReason
		}
		return domain.Video{}, NewConflictError("upload failed: " + reason)
	case domain.VideoStatusUploading:
		// Продолжение обработки ниже.
	}

	return s.completeUploadingVideo(ctx, videoID)
}

// completeUploadingVideo выполняет подтверждение загрузки для видео в статусе uploading:
// проверяет объект в хранилище, регистрирует ассет, переводит видео в очередь и публикует
// событие OriginalUploaded.
func (s *VideoService) completeUploadingVideo(ctx context.Context, videoID uuid.UUID) (domain.Video, error) {
	key := domain.VideoOriginalObjectKey(videoID)

	info, err := s.s3.HeadObject(ctx, s.cfg.Bucket, key)
	if err != nil {
		if errors.Is(err, s3.ErrObjectNotFound) {
			return domain.Video{}, NewConflictError("object not found in storage")
		}
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	if info.Size == 0 {
		return domain.Video{}, NewConflictError("object is empty")
	}

	if _, createErr := s.srv.VideoAsset.Create(
		ctx,
		videoID,
		domain.VideoAssetKindOriginal,
		domain.VideoProfile(""),
		s.cfg.Bucket, key, info.ContentType,
		info.Size,
	); createErr != nil {
		zap.L().Error(createErr.Error())
		return domain.Video{}, createErr
	}

	attempt := videoInitialProcessingAttempt
	updated, err := s.repo.UpdateStatusIf(
		ctx,
		videoID,
		[]domain.VideoStatus{domain.VideoStatusUploading},
		domain.VideoStatusQueued,
		domain.VideoPatch{ProcessingAttempt: &attempt},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	if !updated {
		// Гонка: видео уже переведено в очередь другим вызовом — возвращаем текущее состояние.
		current, selectErr := s.repo.Select(ctx, videoID)
		if selectErr != nil {
			zap.L().Error(selectErr.Error())
			return domain.Video{}, selectErr
		}
		return *current, nil
	}

	if publishErr := s.publishOriginalUploaded(ctx, videoID, attempt, key, info); publishErr != nil {
		return domain.Video{}, publishErr
	}

	result, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	return *result, nil
}

// publishOriginalUploaded собирает и публикует через outbox событие OriginalUploaded
// (§6.1–6.2 эпика) для видео, только что переведённого в очередь на обработку.
func (s *VideoService) publishOriginalUploaded(
	ctx context.Context,
	videoID uuid.UUID,
	attempt int,
	key string,
	info s3.ObjectInfo,
) error {
	envelope, err := events.NewOriginalUploaded(videoID, attempt, events.OriginalUploaded{
		Bucket:      s.cfg.Bucket,
		Key:         key,
		ContentType: info.ContentType,
		SizeBytes:   info.Size,
		Profiles:    s.cfg.Video.Profiles,
	})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	payload, err := envelope.Marshal()
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if publishErr := s.srv.Outbox.Publish(ctx, s.cfg.TopicOriginalUploaded, videoID.String(), payload); publishErr != nil {
		zap.L().Error(publishErr.Error())
		return publishErr
	}

	return nil
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
