package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/hls"
	"vilib-api/internal/repository"
	"vilib-api/internal/s3"
	"vilib-api/internal/saga"

	"github.com/google/uuid"
	"go.uber.org/zap"

	events "github.com/execaus/vilib-events"
)

// videoContentTypePrefix — обязательный префикс MIME-типа загружаемого видео (Э1-Т9, В-9 эпика).
const videoContentTypePrefix = "video/"

// videoInitialProcessingAttempt — номер попытки обработки, с которого видео впервые
// становится в очередь при подтверждении загрузки.
const videoInitialProcessingAttempt = 1

// videoDeleteRetryAttempts — число попыток best-effort удаления объектов хранилища при
// удалении видео/группы (§7.3 дизайна эпика).
const videoDeleteRetryAttempts = 3

// videoDeleteRetryBaseDelay — базовая пауза экспоненциального backoff между повторами
// best-effort удаления объектов хранилища (200ms → 400ms → …).
const videoDeleteRetryBaseDelay = 200 * time.Millisecond

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
	// sleep — функция паузы между повторами best-effort удаления объектов хранилища.
	// В проде — time.Sleep, в тестах подменяется опцией WithDeleteRetrySleep, чтобы не ждать
	// реальные интервалы backoff'а.
	sleep func(time.Duration)
}

// VideoServiceOption настраивает VideoService сверх обязательных зависимостей конструктора.
type VideoServiceOption func(*VideoService)

// WithDeleteRetrySleep подменяет функцию паузы между повторами best-effort удаления объектов
// хранилища (§7.3 дизайна эпика). Предназначена для тестов.
func WithDeleteRetrySleep(sleep func(time.Duration)) VideoServiceOption {
	return func(s *VideoService) {
		s.sleep = sleep
	}
}

func NewVideoService(
	s3 s3.S3,
	repo repository.Video,
	srv *Service,
	cfg VideoServiceConfig,
	opts ...VideoServiceOption,
) *VideoService {
	svc := &VideoService{s3: s3, repo: repo, srv: srv, cfg: cfg, sleep: time.Sleep}

	for _, opt := range opts {
		opt(svc)
	}

	return svc
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

// Get выбирает точку доступа к видео по таблице статус→ответ (§4.4 дизайна эпика): готовое
// видео без предпочтения оригинала при наличии мастер-плейлиста — HLS-токен, иначе, если
// оригинал загружен, — преподписанный URL на оригинал. Если ни один вариант недоступен
// (uploading или failed без оригинала) — ConflictError.
func (s *VideoService) Get(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
	isPreferOriginal bool,
) (domain.VideoAccess, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionVideoWatch,
	); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionVideoWatch); err != nil {
			return domain.VideoAccess{}, ErrForbidden
		}
	}

	// Получение видео по ID
	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAccess{}, err
	}

	// Проверка, что видео принадлежит указанной группе
	if video.GroupID != groupID {
		zap.L().Error("video does not belong to the specified group")
		return domain.VideoAccess{}, ErrForbidden
	}

	// Получение ассетов видео
	assets, err := s.srv.VideoAsset.Get(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAccess{}, err
	}

	master := findAssetByKind(assets, domain.VideoAssetKindHLSMaster)
	if video.Status == domain.VideoStatusReady && !isPreferOriginal && master != nil {
		return s.hlsVideoAccess(*video, assets)
	}

	original := findAssetByKind(assets, domain.VideoAssetKindOriginal)
	if original != nil {
		return s.originalVideoAccess(ctx, *video, original)
	}

	return domain.VideoAccess{}, NewConflictError("video is not available")
}

// hlsVideoAccess собирает точку доступа "hls": выпускает HLS-токен на мастер-плейлист и
// список профилей видео, отсортированных по возрастанию качества.
func (s *VideoService) hlsVideoAccess(video domain.Video, assets []domain.VideoAsset) (domain.VideoAccess, error) {
	token, err := s.srv.Auth.IssueHLSToken(video.ID, s.cfg.Video.HLSURLTTL)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAccess{}, err
	}

	return domain.VideoAccess{
		Kind:      domain.VideoAccessKindHLS,
		HLSToken:  token,
		ExpiresAt: time.Now().Add(s.cfg.Video.HLSURLTTL),
		Video:     video,
		Profiles:  variantProfiles(assets),
	}, nil
}

// originalVideoAccess собирает точку доступа "original": преподписанный URL на GET оригинала.
func (s *VideoService) originalVideoAccess(
	ctx context.Context,
	video domain.Video,
	original *domain.VideoAsset,
) (domain.VideoAccess, error) {
	url, err := s.s3.PresignGetObject(ctx, original.Bucket, original.ObjectKey, domain.VideoStreamURLTTL)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAccess{}, err
	}

	return domain.VideoAccess{
		Kind:      domain.VideoAccessKindOriginal,
		URL:       url,
		ExpiresAt: time.Now().Add(domain.VideoStreamURLTTL),
		Video:     video,
	}, nil
}

// GetHLSMaster проверяет HLS-токен (§4.2 дизайна эпика), убеждается, что видео готово и у
// него есть мастер-плейлист, читает его из хранилища и переписывает URI вариантов на
// относительные ссылки с тем же токеном (§4.3 дизайна эпика).
func (s *VideoService) GetHLSMaster(ctx context.Context, videoID uuid.UUID, token string) ([]byte, error) {
	assets, err := s.hlsRequestAssets(ctx, videoID, token)
	if err != nil {
		return nil, err
	}

	master := findAssetByKind(assets, domain.VideoAssetKindHLSMaster)
	if master == nil {
		zap.L().Warn("hls master asset not found")
		return nil, ErrNotFound
	}

	raw, err := s.s3.GetObject(ctx, master.Bucket, master.ObjectKey)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	rewritten, err := hls.RewriteMaster(raw, func(profile string) string {
		return profile + "/playlist.m3u8?token=" + token
	})
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return rewritten, nil
}

// GetHLSPlaylist проверяет HLS-токен, убеждается, что видео готово и у него есть медиаплейлист
// запрошенного профиля, читает его из хранилища и переписывает имена сегментов на
// преподписанные URL хранилища (§4.2, §4.3 дизайна эпика).
func (s *VideoService) GetHLSPlaylist(
	ctx context.Context,
	videoID uuid.UUID,
	profile domain.VideoProfile,
	token string,
) ([]byte, error) {
	assets, err := s.hlsRequestAssets(ctx, videoID, token)
	if err != nil {
		return nil, err
	}

	variant := findVariantAsset(assets, profile)
	if variant == nil {
		zap.L().Warn("hls variant asset not found")
		return nil, ErrNotFound
	}

	raw, err := s.s3.GetObject(ctx, variant.Bucket, variant.ObjectKey)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// Ключи сегментов лежат рядом с медиаплейлистом профиля (§3.3 дизайна эпика).
	prefix := segmentPrefix(variant.ObjectKey)
	segmentTTL := s.cfg.Video.HLSSegmentTTL

	rewritten, err := hls.RewriteMedia(raw, func(name string) (string, error) {
		url, presignErr := s.s3.PresignGetObject(ctx, variant.Bucket, prefix+name, segmentTTL)
		if presignErr != nil {
			return "", presignErr
		}
		return string(url), nil
	})
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return rewritten, nil
}

// hlsRequestAssets проверяет HLS-токен: подпись/срок/purpose (ParseHLSToken), принадлежность
// токена запрошенному видео и готовность видео (статус ready), затем возвращает его ассеты
// для поиска нужного плейлиста (§4.2 дизайна эпика).
func (s *VideoService) hlsRequestAssets(
	ctx context.Context,
	videoID uuid.UUID,
	token string,
) ([]domain.VideoAsset, error) {
	claims, err := s.srv.Auth.ParseHLSToken(token)
	if err != nil {
		return nil, err
	}

	if claims.VideoID != videoID {
		zap.L().Warn("hls token video id mismatch")
		return nil, ErrForbidden
	}

	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	if video.Status != domain.VideoStatusReady {
		return nil, NewConflictError("video is not available")
	}

	assets, err := s.srv.VideoAsset.Get(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assets, nil
}

// findVariantAsset ищет ассет hls_variant с указанным профилем. Возвращает nil, если такого нет.
func findVariantAsset(assets []domain.VideoAsset, profile domain.VideoProfile) *domain.VideoAsset {
	for i := range assets {
		if assets[i].Kind == domain.VideoAssetKindHLSVariant && assets[i].Profile == profile {
			return &assets[i]
		}
	}

	return nil
}

// variantProfiles собирает имена профилей hls_variant-ассетов, отсортированные по возрастанию
// числовой части имени ("360p" < "720p" < "1080p").
func variantProfiles(assets []domain.VideoAsset) []string {
	var profiles []string

	for _, asset := range assets {
		if asset.Kind == domain.VideoAssetKindHLSVariant {
			profiles = append(profiles, string(asset.Profile))
		}
	}

	sort.Slice(profiles, func(i, j int) bool {
		return profileNumericPrefix(profiles[i]) < profileNumericPrefix(profiles[j])
	})

	return profiles
}

// profileNumericPrefix извлекает числовую часть имени профиля ("720p" → 720). Профили без
// числового префикса сортируются как 0.
func profileNumericPrefix(profile string) int {
	end := 0
	for end < len(profile) && profile[end] >= '0' && profile[end] <= '9' {
		end++
	}

	n, _ := strconv.Atoi(profile[:end])

	return n
}

// segmentPrefix вычисляет префикс ключей сегментов из ключа медиаплейлиста: тот же путь без
// последнего сегмента (§3.3 дизайна эпика — сегменты лежат рядом с плейлистом профиля).
func segmentPrefix(playlistKey string) string {
	idx := strings.LastIndex(playlistKey, "/")
	if idx < 0 {
		return ""
	}

	return playlistKey[:idx+1]
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
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		userID,
		domain.AccountPermissionManageVideo,
	); err != nil {
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
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		userID,
		domain.AccountPermissionManageVideo,
	); err != nil {
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

	if publishErr := s.srv.Outbox.Publish(
		ctx,
		s.cfg.TopicOriginalUploaded,
		videoID.String(),
		payload,
	); publishErr != nil {
		zap.L().Error(publishErr.Error())
		return publishErr
	}

	return nil
}

// processingStatuses — статусы, из которых допустимы переходы по событиям обработки воркера
// (§1.3 эпика: диаграмма состояний видео).
func processingStatuses() []domain.VideoStatus {
	return []domain.VideoStatus{domain.VideoStatusQueued, domain.VideoStatusCompressing}
}

// validProcessingResultKind проверяет, что вид ассета из результата обработки допустим для
// регистрации по ProcessingCompleted (оригинал регистрируется отдельно при CompleteUpload).
func validProcessingResultKind(kind string) (domain.VideoAssetKind, bool) {
	switch domain.VideoAssetKind(kind) {
	case domain.VideoAssetKindHLSMaster, domain.VideoAssetKindHLSVariant:
		return domain.VideoAssetKind(kind), true
	case domain.VideoAssetKindOriginal:
		return "", false
	default:
		return "", false
	}
}

// ApplyProcessingStarted переводит видео из очереди в обработку по событию ProcessingStarted
// воркера (§7.2 эпика). Системный вызов без проверки прав. Переход выполняется условным
// UPDATE (queued → compressing, только при совпадении номера попытки); если строка не
// обновилась — переход недопустим (устаревшая попытка, повторный ProcessingStarted, гонка с
// watchdog'ом) и молча игнорируется с логом.
func (s *VideoService) ApplyProcessingStarted(
	ctx context.Context,
	evt events.Envelope,
	_ events.ProcessingStarted,
) error {
	attempt := evt.Attempt

	updated, err := s.repo.UpdateStatusIf(
		ctx,
		evt.VideoID,
		[]domain.VideoStatus{domain.VideoStatusQueued},
		domain.VideoStatusCompressing,
		domain.VideoPatch{ExpectedAttempt: &attempt},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if !updated {
		zap.L().Info("processing started transition ignored",
			zap.String("video_id", evt.VideoID.String()),
			zap.Int("attempt", evt.Attempt),
		)
	}

	return nil
}

// ApplyProcessingCompleted обрабатывает событие ProcessingCompleted (§7.2 эпика). Переход в
// ready выполняется условным UPDATE первым: только если он применился (видео было в
// queued/compressing и номер попытки совпал), в этой же транзакции идемпотентно
// перерегистрируются ассеты результатов — старые hls_master/hls_variant удаляются и
// вставляются заново (Э1-Т14). Если переход не применился (видео не найдено, статус вне
// ожидаемого набора или попытка устарела), новые ассеты не регистрируются: после коммита
// транзакции best-effort зачищаются возможные результаты-сироты в хранилище (Э1-Т22).
func (s *VideoService) ApplyProcessingCompleted(
	ctx context.Context,
	evt events.Envelope,
	p events.ProcessingCompleted,
) error {
	attempt := evt.Attempt

	patch := domain.VideoPatch{ExpectedAttempt: &attempt, ClearFailure: true}
	if p.Metadata.DurationMs > 0 {
		patch.DurationMs = &p.Metadata.DurationMs
	}
	if p.Metadata.Width > 0 {
		patch.Width = &p.Metadata.Width
	}
	if p.Metadata.Height > 0 {
		patch.Height = &p.Metadata.Height
	}

	updated, err := s.repo.UpdateStatusIf(ctx, evt.VideoID, processingStatuses(), domain.VideoStatusReady, patch)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if !updated {
		zap.L().Info("processing completed event ignored: stale attempt or video not in processing status",
			zap.String("video_id", evt.VideoID.String()),
			zap.Int("attempt", evt.Attempt),
		)
		s.cleanupOrphanProcessingResults(ctx, evt.VideoID)
		return nil
	}

	return s.registerProcessingResults(ctx, evt.VideoID, p.Results)
}

// registerProcessingResults идемпотентно перерегистрирует ассеты результатов обработки видео:
// удаляет ранее зарегистрированные hls_master/hls_variant и вставляет присланные заново.
func (s *VideoService) registerProcessingResults(
	ctx context.Context,
	videoID uuid.UUID,
	results []events.AssetResult,
) error {
	if err := s.srv.VideoAsset.DeleteByVideoAndKinds(
		ctx,
		videoID,
		[]domain.VideoAssetKind{domain.VideoAssetKindHLSMaster, domain.VideoAssetKindHLSVariant},
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, result := range results {
		kind, ok := validProcessingResultKind(result.Kind)
		if !ok {
			err := fmt.Errorf("%w: unsupported processing result asset kind %q", ErrValidation, result.Kind)
			zap.L().Error(err.Error())
			return err
		}

		if _, createErr := s.srv.VideoAsset.Create(
			ctx,
			videoID,
			kind,
			domain.VideoProfile(result.Profile),
			result.Bucket, result.Key, result.ContentType,
			result.SizeBytes,
		); createErr != nil {
			zap.L().Error(createErr.Error())
			return createErr
		}
	}

	return nil
}

// cleanupOrphanProcessingResults регистрирует best-effort зачистку результатов обработки
// в хранилище после коммита транзакции (§7.3 эпика) — на случай, если воркер уже успел
// загрузить объекты по устаревшему/отклонённому событию (удалённое видео, гонка с watchdog'ом).
func (s *VideoService) cleanupOrphanProcessingResults(ctx context.Context, videoID uuid.UUID) {
	prefix := domain.VideoHLSPrefix(videoID)

	saga.AfterCommit(ctx, func(hookCtx context.Context) {
		if _, err := s.s3.DeleteByPrefix(hookCtx, s.cfg.Bucket, prefix); err != nil {
			zap.L().Error("failed to cleanup orphan processing results",
				zap.String("video_id", videoID.String()),
				zap.String("prefix", prefix),
				zap.Error(err),
			)
		}
	})
}

// ApplyProcessingFailed обрабатывает событие ProcessingFailed (§7.2 эпика). Постоянная ошибка
// или временная с исчерпанными попытками переводят видео в failed; временная ошибка с запасом
// попыток возвращает видео в очередь с увеличенным номером попытки и публикует повторное
// событие OriginalUploaded. Неизвестный класс ошибки трактуется как временный. Переход
// выполняется условным UPDATE — недопустимый (видео не найдено, статус вне ожидаемого набора,
// устаревшая попытка) молча игнорируется с логом.
func (s *VideoService) ApplyProcessingFailed(
	ctx context.Context,
	evt events.Envelope,
	p events.ProcessingFailed,
) error {
	if p.ErrorClass == events.ErrorClassPermanent {
		return s.failProcessing(ctx, evt.VideoID, evt.Attempt, domain.VideoFailureClassPermanent, p.Reason)
	}

	if evt.Attempt < s.cfg.Video.MaxProcessingAttempts {
		return s.requeueAfterTemporaryFailure(ctx, evt.VideoID, evt.Attempt)
	}

	return s.failProcessing(
		ctx, evt.VideoID, evt.Attempt, domain.VideoFailureClassTemporary, "attempts exhausted: "+p.Reason,
	)
}

// failProcessing условно переводит видео в failed с указанным классом и причиной ошибки.
func (s *VideoService) failProcessing(
	ctx context.Context,
	videoID uuid.UUID,
	attempt int,
	class domain.VideoFailureClass,
	reason string,
) error {
	updated, err := s.repo.UpdateStatusIf(
		ctx,
		videoID,
		processingStatuses(),
		domain.VideoStatusFailed,
		domain.VideoPatch{ExpectedAttempt: &attempt, FailureClass: &class, FailureReason: &reason},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if !updated {
		zap.L().Info("processing failed transition ignored: stale attempt or video not in processing status",
			zap.String("video_id", videoID.String()),
			zap.Int("attempt", attempt),
		)
	}

	return nil
}

// requeueAfterTemporaryFailure условно возвращает видео из обработки в очередь с очередным
// номером попытки и публикует повторное событие OriginalUploaded по данным уже
// зарегистрированного ассета-оригинала.
func (s *VideoService) requeueAfterTemporaryFailure(ctx context.Context, videoID uuid.UUID, attempt int) error {
	next := attempt + 1

	updated, err := s.repo.UpdateStatusIf(
		ctx,
		videoID,
		processingStatuses(),
		domain.VideoStatusQueued,
		domain.VideoPatch{ExpectedAttempt: &attempt, ProcessingAttempt: &next},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if !updated {
		zap.L().Info("temporary failure requeue ignored: stale attempt or video not in processing status",
			zap.String("video_id", videoID.String()),
			zap.Int("attempt", attempt),
		)
		return nil
	}

	assets, err := s.srv.VideoAsset.Get(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	original := findAssetByKind(assets, domain.VideoAssetKindOriginal)
	if original == nil {
		notFoundErr := fmt.Errorf("original asset not found for video %s", videoID)
		zap.L().Error(notFoundErr.Error())
		return notFoundErr
	}

	return s.publishOriginalUploaded(ctx, videoID, next, original.ObjectKey, s3.ObjectInfo{
		Size:        original.SizeBytes,
		ContentType: original.ContentType,
	})
}

// GetAll возвращает список видео группы (Э1-Т20, §5 дизайна эпика): профили и признак
// обработки вычисляются из ассетов видео, причина сбоя (Failure) заполняется только для
// инициатора с правом ManageVideo (аккаунтным или групповым) — иначе остаётся nil даже у
// видео в статусе failed (Э1-Т17).
func (s *VideoService) GetAll(
	ctx context.Context,
	accountID, groupID, initiatorID uuid.UUID,
) ([]domain.VideoListItem, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionVideoWatch,
	); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionVideoWatch); err != nil {
			return nil, ErrForbidden
		}
	}

	canManage := s.canManageVideo(ctx, accountID, groupID, initiatorID)

	// Получение списка видео группы
	videos, err := s.repo.SelectByGroupID(ctx, groupID)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	videoIDs := make([]uuid.UUID, len(videos))
	for i, video := range videos {
		videoIDs[i] = video.ID
	}

	assets, err := s.srv.VideoAsset.SelectByVideoIDs(ctx, videoIDs)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	assetsByVideo := make(map[uuid.UUID][]domain.VideoAsset, len(videos))
	for _, asset := range assets {
		assetsByVideo[asset.VideoID] = append(assetsByVideo[asset.VideoID], asset)
	}

	items := make([]domain.VideoListItem, len(videos))
	for i, video := range videos {
		items[i] = newVideoListItem(video, assetsByVideo[video.ID], canManage)
	}

	return items, nil
}

// newVideoListItem собирает элемент списка видео (Э1-Т20): профили и признак обработки — из
// ассетов видео, причина сбоя — только если canManage и у видео есть класс ошибки (Э1-Т17).
func newVideoListItem(video domain.Video, assets []domain.VideoAsset, canManage bool) domain.VideoListItem {
	item := domain.VideoListItem{
		Video:        video,
		Profiles:     variantProfiles(assets),
		HasProcessed: findAssetByKind(assets, domain.VideoAssetKindHLSMaster) != nil,
	}

	if canManage && video.FailureClass != nil {
		reason := ""
		if video.FailureReason != nil {
			reason = *video.FailureReason
		}
		item.Failure = &domain.VideoFailure{Class: *video.FailureClass, Reason: reason}
	}

	return item
}

// canManageVideo определяет, доступно ли инициатору право ManageVideo — аккаунтное или
// групповое (OR-логика) — без возврата ошибки. Используется там, где отсутствие права не
// запрещает действие целиком, а лишь скрывает часть ответа (Э1-Т17: причина сбоя видна только
// с ManageVideo).
func (s *VideoService) canManageVideo(ctx context.Context, accountID, groupID, initiatorID uuid.UUID) bool {
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageVideo,
	); err == nil {
		return true
	}

	return s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionManageVideo) == nil
}

func (s *VideoService) Rename(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
	name string,
) (domain.Video, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageVideo,
	); err != nil {
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
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		initiatorID,
		domain.AccountPermissionManageVideo,
	); err != nil {
		if err := s.isCheckGroupAction(ctx, groupID, initiatorID, domain.GroupPermissionManageVideo); err != nil {
			return ErrForbidden
		}
	}

	// Удаление видео: сначала БД-транзакция, объекты хранилища — после коммита (Э1-Т21).
	if err := s.repo.Delete(ctx, videoID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	s.DeleteObjectsAfterCommit(ctx, videoID)

	return nil
}

// DeleteObjectsAfterCommit регистрирует best-effort зачистку всех объектов перечисленных видео
// в хранилище (videos/{id}/ — единый префикс оригинала и всех результатов обработки, §3.3
// дизайна эпика) после успешного коммита транзакции саги (§7.3, Э1-Т21). Вызывается как из
// Video.Delete, так и из UserGroup.Delete (id видео группы известны до каскадного удаления
// строк БД). Порядок регистрации хука относительно момента, когда видео удалены из БД, не
// важен — хук выполняется только после коммита всей транзакции.
func (s *VideoService) DeleteObjectsAfterCommit(ctx context.Context, videoIDs ...uuid.UUID) {
	for _, videoID := range videoIDs {
		prefix := domain.VideoPrefix(videoID)

		saga.AfterCommit(ctx, func(hookCtx context.Context) {
			s.deleteObjectsWithRetry(hookCtx, prefix)
		})
	}
}

// deleteObjectsWithRetry выполняет best-effort удаление всех объектов под префиксом с
// повторами и экспоненциальным backoff'ом (§7.3 дизайна эпика). К моменту вызова БД-строки
// уже удалены и закоммичены — неудача всех попыток означает лишь сироту в S3, что допустимо
// и логируется (Э1-Т21), саму (уже завершённую) транзакцию она не откатывает.
func (s *VideoService) deleteObjectsWithRetry(ctx context.Context, prefix string) {
	delay := videoDeleteRetryBaseDelay
	var lastErr error

	for attempt := 1; attempt <= videoDeleteRetryAttempts; attempt++ {
		if _, err := s.s3.DeleteByPrefix(ctx, s.cfg.Bucket, prefix); err != nil {
			lastErr = err
			if attempt < videoDeleteRetryAttempts {
				s.sleep(delay)
				delay *= 2
			}
			continue
		}
		return
	}

	zap.L().Error("orphan objects in storage: delete retries exhausted",
		zap.String("prefix", prefix),
		zap.Error(lastErr),
	)
}

// FailTimedOut переводит в failed(timeout) видео, зависшие в uploading/queued/compressing
// дольше сконфигурированных таймаутов (§8 дизайна эпика, Э1-Т16). Вызывается watchdog'ом на
// каждом тике. Каждый переход — один атомарный условный UPDATE (repository.Video.UpdateTimedOut),
// поэтому метод безопасен при нескольких одновременно работающих инстансах API: строку,
// которую уже перевёл другой инстанс, повторный UPDATE не затронет (WHERE status = <исходный
// статус> перестаёт совпадать) — гонки не возникает.
func (s *VideoService) FailTimedOut(ctx context.Context, now time.Time) (domain.TimedOutReport, error) {
	uploading, err := s.repo.UpdateTimedOut(
		ctx,
		domain.VideoStatusUploading,
		now.Add(-s.cfg.Video.UploadTimeout),
		domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: fmt.Sprintf("загрузка не завершена за %s", s.cfg.Video.UploadTimeout),
		},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.TimedOutReport{}, err
	}

	// Оригинал мог появиться в хранилище частично или уже после того, как видео признано
	// зависшим, — очищаем его best-effort после коммита (§8 дизайна эпика).
	for _, videoID := range uploading {
		key := domain.VideoOriginalObjectKey(videoID)

		saga.AfterCommit(ctx, func(hookCtx context.Context) {
			if deleteErr := s.s3.DeleteObject(hookCtx, s.cfg.Bucket, key); deleteErr != nil &&
				!errors.Is(deleteErr, s3.ErrObjectNotFound) {
				zap.L().Error("failed to cleanup timed out upload object",
					zap.String("video_id", videoID.String()),
					zap.String("key", key),
					zap.Error(deleteErr),
				)
			}
		})
	}

	queued, err := s.repo.UpdateTimedOut(
		ctx,
		domain.VideoStatusQueued,
		now.Add(-s.cfg.Video.QueuedTimeout),
		domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: fmt.Sprintf("не взято в обработку за %s", s.cfg.Video.QueuedTimeout),
		},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.TimedOutReport{}, err
	}

	// compressing: поздний ProcessingCompleted для перешедшего в failed видео игнорируется и
	// его результаты зачищаются логикой ApplyProcessingCompleted (§7.2 эпика) — здесь очищать
	// хранилище не нужно.
	compressing, err := s.repo.UpdateTimedOut(
		ctx,
		domain.VideoStatusCompressing,
		now.Add(-s.cfg.Video.ProcessingTimeout),
		domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: fmt.Sprintf("обработка не завершена за %s", s.cfg.Video.ProcessingTimeout),
		},
	)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.TimedOutReport{}, err
	}

	zap.L().Info("watchdog tick completed",
		zap.Int("uploading_timed_out", len(uploading)),
		zap.Int("queued_timed_out", len(queued)),
		zap.Int("compressing_timed_out", len(compressing)),
	)

	return domain.TimedOutReport{Uploading: uploading, Queued: queued, Compressing: compressing}, nil
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
