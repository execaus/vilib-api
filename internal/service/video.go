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
	// TopicOriginalUploaded — топик Kafka архивной полосы для публикации события
	// OriginalUploaded несрочных видео.
	TopicOriginalUploaded string
	// TopicOriginalUploadedUrgent — топик Kafka приоритетной полосы для публикации события
	// OriginalUploaded видео, помеченных срочными (эпик Э5, В-2).
	TopicOriginalUploadedUrgent string
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
	// pipelineProgress — индикатор живости конвейера обработки видео по полосам (эпик Э5,
	// исправление Д-1): используется watchdog'ом (FailTimedOut) и обработчиком ProcessingStarted
	// (ApplyProcessingStarted), задаётся опцией WithPipelineProgress.
	pipelineProgress repository.PipelineProgress
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

// WithPipelineProgress задаёт репозиторий индикатора живости конвейера по полосам (эпик Э5,
// исправление Д-1) — обязателен в проде для FailTimedOut и ApplyProcessingStarted.
func WithPipelineProgress(repo repository.PipelineProgress) VideoServiceOption {
	return func(s *VideoService) {
		s.pipelineProgress = repo
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
	if err := s.canWatch(ctx, accountID, groupID, initiatorID); err != nil {
		return domain.VideoAccess{}, err
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

	return domain.VideoAccess{}, ErrVideoNotAvailable
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
		return nil, ErrVideoNotAvailable
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
// оригинала в хранилище (Э1-Т7, §5 дизайна эпика). isUrgent помечает видео срочным (эпик Э5,
// В-2) — такое видео будет опубликовано в приоритетную полосу обработки при подтверждении
// загрузки.
func (s *VideoService) CreateUpload(
	ctx context.Context,
	accountID, groupID, userID uuid.UUID,
	name, contentType string,
	size int64,
	isUrgent bool,
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
		return domain.VideoUpload{}, NewValidationErrorCode(
			codeValidationContentType,
			"content_type must start with "+videoContentTypePrefix,
		)
	}
	if size <= 0 || size > s.cfg.Video.MaxUploadSizeBytes {
		return domain.VideoUpload{}, NewValidationErrorCode(
			codeValidationSizeBytes,
			"size_bytes must be between 1 and max upload size",
		)
	}

	// Создание записи о видео в статусе загрузки
	video, err := s.repo.Insert(ctx, name, groupID, userID, domain.VideoStatusUploading, isUrgent)
	if err != nil {
		if errors.Is(dberrors.UserGroupVideoErrors.ErrUniqueUserGroupVideosUserGroupIdNameKey, err) {
			zap.L().Warn(err.Error())
			return domain.VideoUpload{}, ErrVideoNameExists
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
// Возвращает карточку того же вида, что и элемент списка видео (VideoListItem с профилями и
// автором, §5.1 контракта Э2).
func (s *VideoService) CompleteUpload(
	ctx context.Context,
	accountID, groupID, userID, videoID uuid.UUID,
) (domain.VideoListItem, error) {
	// OR-логика: аккаунтное право ИЛИ групповое право
	if err := s.srv.Access.IsCheckAccountAction(
		ctx,
		accountID,
		userID,
		domain.AccountPermissionManageVideo,
	); err != nil {
		if groupErr := s.isCheckGroupAction(ctx, groupID, userID, domain.GroupPermissionManageVideo); groupErr != nil {
			return domain.VideoListItem{}, ErrForbidden
		}
	}

	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoListItem{}, err
	}

	// Проверка, что видео принадлежит указанной группе
	if video.GroupID != groupID {
		zap.L().Error("video does not belong to the specified group")
		return domain.VideoListItem{}, ErrForbidden
	}

	switch video.Status {
	case domain.VideoStatusQueued, domain.VideoStatusCompressing, domain.VideoStatusReady:
		// Повторное подтверждение уже принятой загрузки — идемпотентный no-op. Право
		// ManageVideo уже подтверждено проверкой выше — повторно не проверяем.
		return s.videoListItemWithAuthor(ctx, *video, true)
	case domain.VideoStatusFailed:
		reason := "timeout"
		if video.FailureReason != nil {
			reason = *video.FailureReason
		}
		return domain.VideoListItem{}, NewConflictErrorCode(codeConflictUploadFailed, "upload failed: "+reason)
	case domain.VideoStatusUploading:
		// Продолжение обработки ниже.
	}

	result, err := s.completeUploadingVideo(ctx, videoID, video.IsUrgent)
	if err != nil {
		return domain.VideoListItem{}, err
	}

	return s.videoListItemWithAuthor(ctx, result, true)
}

// completeUploadingVideo выполняет подтверждение загрузки для видео в статусе uploading:
// проверяет объект в хранилище, регистрирует ассет, переводит видео в очередь (проставляя
// queued_at — момент complete из метрики времени публикации, Э5-Т5) и публикует событие
// OriginalUploaded в полосу, соответствующую isUrgent (эпик Э5, В-2).
func (s *VideoService) completeUploadingVideo(
	ctx context.Context,
	videoID uuid.UUID,
	isUrgent bool,
) (domain.Video, error) {
	key := domain.VideoOriginalObjectKey(videoID)

	info, err := s.s3.HeadObject(ctx, s.cfg.Bucket, key)
	if err != nil {
		if errors.Is(err, s3.ErrObjectNotFound) {
			return domain.Video{}, ErrObjectNotFoundInStorage
		}
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	if info.Size == 0 {
		return domain.Video{}, ErrObjectEmpty
	}

	if _, createErr := s.srv.VideoAsset.Create(
		ctx,
		videoID,
		domain.VideoAssetKindOriginal,
		domain.VideoProfile(""),
		s.cfg.Bucket, key, info.ContentType,
		info.Size,
	); createErr != nil {
		if isOriginalAssetConflict(createErr) {
			// Гонка двух одновременных complete для одного видео (Д-5 ревью эпика): оба прошли
			// HeadObject, второй проигрывает на уникальном ограничении при регистрации
			// ассета-оригинала (ключ объекта детерминирован — совпадает и в files, и в
			// video_assets). Идемпотентно возвращаем текущее состояние вместо 500.
			zap.L().Warn("concurrent complete upload: original asset already registered by another request",
				zap.String("video_id", videoID.String()),
			)
			current, selectErr := s.repo.Select(ctx, videoID)
			if selectErr != nil {
				zap.L().Error(selectErr.Error())
				return domain.Video{}, selectErr
			}
			return *current, nil
		}
		zap.L().Error(createErr.Error())
		return domain.Video{}, createErr
	}

	attempt := videoInitialProcessingAttempt
	queuedAt := time.Now()
	updated, err := s.repo.UpdateStatusIf(
		ctx,
		videoID,
		[]domain.VideoStatus{domain.VideoStatusUploading},
		domain.VideoStatusQueued,
		domain.VideoPatch{ProcessingAttempt: &attempt, QueuedAt: &queuedAt},
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

	if publishErr := s.publishOriginalUploaded(ctx, videoID, attempt, isUrgent, key, info); publishErr != nil {
		return domain.Video{}, publishErr
	}

	result, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	return *result, nil
}

// isOriginalAssetConflict определяет нарушение уникальности при регистрации ассета-оригинала —
// сигнал гонки двух одновременных подтверждений загрузки одного видео (Д-5 ревью эпика).
// VideoAsset.Create вставляет две строки одной операцией (files, затем video_assets); ключ
// объекта оригинала детерминирован (VideoOriginalObjectKey), поэтому в зависимости от того, на
// каком шаге столкнулись конкурирующие запросы, конфликт может прийти по любому из двух
// уникальных ограничений — проверяются оба.
func isOriginalAssetConflict(err error) bool {
	return errors.Is(dberrors.FileErrors.ErrUniqueFilesBucketObjectKeyKey, err) ||
		errors.Is(dberrors.VideoAssetErrors.ErrUniqueVideoAssetsVideoIdKindProfileKey, err)
}

// publishOriginalUploaded собирает и публикует через outbox событие OriginalUploaded
// (§6.1–6.2 эпика) для видео, только что переведённого в очередь на обработку. Топик
// выбирается по isUrgent (эпик Э5, В-2, §2 дизайна эпика): срочное видео публикуется в
// приоритетную полосу с постоянно свободным потребителем, а не в общий архивный топик —
// маршрутизация определяется топиком, поле IsUrgent в самом событии нужно только для
// наблюдаемости на стороне воркера.
func (s *VideoService) publishOriginalUploaded(
	ctx context.Context,
	videoID uuid.UUID,
	attempt int,
	isUrgent bool,
	key string,
	info s3.ObjectInfo,
) error {
	envelope, err := events.NewOriginalUploaded(videoID, attempt, events.OriginalUploaded{
		Bucket:      s.cfg.Bucket,
		Key:         key,
		ContentType: info.ContentType,
		SizeBytes:   info.Size,
		Profiles:    s.cfg.Video.Profiles,
		IsUrgent:    isUrgent,
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

	topic := s.cfg.TopicOriginalUploaded
	if isUrgent {
		topic = s.cfg.TopicOriginalUploadedUrgent
	}

	if publishErr := s.srv.Outbox.Publish(
		ctx,
		topic,
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
// watchdog'ом) и молча игнорируется с логом. Если переход применился — в той же транзакции
// обновляется индикатор живости полосы обработки видео (эпик Э5, исправление Д-1): watchdog
// видит полосу живой, пока хотя бы одно видео этой полосы реально берётся в обработку, вне
// зависимости от длины очереди перед ним.
func (s *VideoService) ApplyProcessingStarted(
	ctx context.Context,
	evt events.Envelope,
	_ events.ProcessingStarted,
) error {
	attempt := evt.Attempt
	now := time.Now()

	updated, err := s.repo.UpdateStatusIf(
		ctx,
		evt.VideoID,
		[]domain.VideoStatus{domain.VideoStatusQueued},
		domain.VideoStatusCompressing,
		domain.VideoPatch{ExpectedAttempt: &attempt, CompressingStartedAt: &now},
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
		return nil
	}

	video, err := s.repo.Select(ctx, evt.VideoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if err = s.pipelineProgress.UpdateLastDequeuedAt(ctx, video.IsUrgent, now); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// ApplyProcessingCompleted обрабатывает событие ProcessingCompleted (§7.2 эпика). Список
// результатов валидируется до какого-либо UPDATE (Д-3 ревью эпика): неизвестный вид ассета или
// повторная пара (kind, profile) — poison-событие (иначе `UNIQUE(video_id, kind, profile)`
// уронит вставку и обработчик уйдёт в бесконечный ретрай), поэтому такое видео сразу
// переводится в failed(permanent) и событие подтверждается (return nil). Иначе переход в ready
// выполняется условным UPDATE: только если он применился (видео было в queued/compressing и
// номер попытки совпал), в этой же транзакции идемпотентно перерегистрируются ассеты
// результатов — старые hls_master/hls_variant удаляются и вставляются заново (Э1-Т14). Если
// переход не применился, новые ассеты не регистрируются — обрабатывается как дубликат/устаревшее
// событие (Д-1 ревью эпика, см. handleIgnoredProcessingCompleted). При успешном переходе также
// проставляется ready_at — финальная отметка метрики времени публикации (эпик Э5, Э5-Т5) — и
// пишется структурированный лог с разбивкой ожидания в очереди и времени кодирования
// (logVideoPublished).
func (s *VideoService) ApplyProcessingCompleted(
	ctx context.Context,
	evt events.Envelope,
	p events.ProcessingCompleted,
) error {
	if validationErr := validateProcessingResults(p.Results); validationErr != nil {
		zap.L().Warn("processing completed event is poison: invalid results, failing video",
			zap.String("video_id", evt.VideoID.String()),
			zap.Int("attempt", evt.Attempt),
			zap.Error(validationErr),
		)
		return s.failProcessing(
			ctx, evt.VideoID, evt.Attempt,
			domain.VideoFailureClassPermanent, "invalid processing result: "+validationErr.Error(),
		)
	}

	attempt := evt.Attempt
	readyAt := time.Now()

	patch := domain.VideoPatch{ExpectedAttempt: &attempt, ClearFailure: true, ReadyAt: &readyAt}
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
		return s.handleIgnoredProcessingCompleted(ctx, evt)
	}

	return s.finalizeReadyVideo(ctx, evt, p, readyAt)
}

// finalizeReadyVideo выполняет шаги, следующие за успешным условным переходом видео в ready:
// регистрирует результаты обработки, досчитывает зачёт по длительности и пишет метрику времени
// публикации (эпик Э5, Э5-Т5). Вынесено из ApplyProcessingCompleted отдельным методом, чтобы не
// наращивать вложенность условий в основном обработчике события.
func (s *VideoService) finalizeReadyVideo(
	ctx context.Context,
	evt events.Envelope,
	p events.ProcessingCompleted,
	readyAt time.Time,
) error {
	if resultsErr := s.registerProcessingResults(ctx, evt.VideoID, p.Results); resultsErr != nil {
		return resultsErr
	}

	// Досчёт зачёта для тех, кто уже посмотрел видео на нужную долю до появления длительности
	// (Э3-Т6, §3 дизайна эпика Э3) — только если она пришла в этом событии.
	if p.Metadata.DurationMs > 0 {
		if durationErr := s.srv.WatchProgress.OnDurationKnown(
			ctx,
			evt.VideoID,
			p.Metadata.DurationMs,
		); durationErr != nil {
			zap.L().Error(durationErr.Error())
			return durationErr
		}
	}

	return s.logVideoPublished(ctx, evt, p, readyAt)
}

// logVideoPublished пишет структурированный лог факта публикации видео (эпик Э5, Э5-Т5) —
// страховка для наблюдения на живом стенде без похода в БД (`docker compose logs`); источником
// истины для методики замера остаются колонки queued_at/compressing_started_at/ready_at (§4
// дизайна эпика), лог их дублирует уже посчитанными разностями. Видео перечитывается в той же
// транзакции, что и переход в ready, — queued_at и compressing_started_at к этому моменту уже
// заполнены более ранними шагами конвейера (В-73/В-74), is_urgent — постоянный признак видео.
// Ошибка перечитывания видео пробрасывается вызывающему: обработчик события идемпотентен
// (handleIgnoredProcessingCompleted), поэтому повтор доставки безопасен.
func (s *VideoService) logVideoPublished(
	ctx context.Context,
	evt events.Envelope,
	p events.ProcessingCompleted,
	readyAt time.Time,
) error {
	video, err := s.repo.Select(ctx, evt.VideoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	zap.L().Info("video published",
		zap.String("video_id", evt.VideoID.String()),
		zap.Int64("duration_ms", p.Metadata.DurationMs),
		zap.Int64("queue_wait_ms", msBetween(video.QueuedAt, video.CompressingStartedAt)),
		zap.Int64("encode_ms", msBetween(video.CompressingStartedAt, &readyAt)),
		zap.Int64("total_ms", msBetween(video.QueuedAt, &readyAt)),
		zap.Bool("is_urgent", video.IsUrgent),
		zap.Int("attempt", evt.Attempt),
	)

	return nil
}

// msBetween вычисляет разницу между двумя моментами в миллисекундах для метрики времени
// публикации (эпик Э5, Э5-Т5). Если любая из отметок не заполнена (гонка/дефект более раннего
// шага конвейера), возвращает 0 вместо паники — лог деградирует наглядностью, а не падает.
func msBetween(start, end *time.Time) int64 {
	if start == nil || end == nil {
		return 0
	}

	return end.Sub(*start).Milliseconds()
}

// handleIgnoredProcessingCompleted обрабатывает ProcessingCompleted, не применившийся условным
// UPDATE в ready (Д-1 ревью эпика). Видео перечитывается, чтобы отличить настоящую сироту от
// штатного дубликата at-least-once доставки:
//   - видео не найдено (удалено) — сирота, HLS-результаты, которые воркер мог успеть залить,
//     зачищаются best-effort после коммита (Э1-Т22);
//   - видео уже failed — тоже сирота (например, watchdog победил гонку с этим же событием) —
//     зачистка;
//   - в остальных случаях (video ready — дубликат этого же события; video queued/compressing —
//     устаревшая попытка или гонка с watchdog'ом) — no-op без очистки: для ready актуальные
//     объекты только что залил победивший воркер, их удаление стёрло бы рабочее видео; для
//     queued/compressing актуальный воркер сам чистит префикс перед своей загрузкой.
func (s *VideoService) handleIgnoredProcessingCompleted(ctx context.Context, evt events.Envelope) error {
	video, err := s.repo.Select(ctx, evt.VideoID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			zap.L().Info("processing completed event ignored: video not found, cleaning up orphan results",
				zap.String("video_id", evt.VideoID.String()),
				zap.Int("attempt", evt.Attempt),
			)
			s.cleanupOrphanProcessingResults(ctx, evt.VideoID)
			return nil
		}
		zap.L().Error(err.Error())
		return err
	}

	if video.Status == domain.VideoStatusFailed {
		zap.L().Info("processing completed event ignored: video already failed, cleaning up orphan results",
			zap.String("video_id", evt.VideoID.String()),
			zap.Int("attempt", evt.Attempt),
		)
		s.cleanupOrphanProcessingResults(ctx, evt.VideoID)
		return nil
	}

	zap.L().Info("processing completed event ignored: duplicate delivery or stale attempt, no cleanup",
		zap.String("video_id", evt.VideoID.String()),
		zap.Int("attempt", evt.Attempt),
		zap.String("status", video.Status.String()),
	)

	return nil
}

// validateProcessingResults проверяет список результатов ProcessingCompleted до какой-либо
// записи в БД (Д-3 ревью эпика): вид ассета должен быть из допустимого набора, а пара
// (kind, profile) не должна повторяться в пределах события — иначе вставка упадёт на
// `UNIQUE(video_id, kind, profile)` (Э1-Т14) уже после того, как старые ассеты удалены
// registerProcessingResults, оставляя видео без результатов вовсе.
func validateProcessingResults(results []events.AssetResult) error {
	seen := make(map[string]struct{}, len(results))

	for _, result := range results {
		kind, ok := validProcessingResultKind(result.Kind)
		if !ok {
			return fmt.Errorf("unsupported asset kind %q", result.Kind)
		}

		key := string(kind) + "/" + result.Profile
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate result for kind %q profile %q", kind, result.Profile)
		}
		seen[key] = struct{}{}
	}

	return nil
}

// registerProcessingResults идемпотентно перерегистрирует ассеты результатов обработки видео:
// удаляет ранее зарегистрированные hls_master/hls_variant и вставляет присланные заново. Список
// результатов уже прошёл validateProcessingResults — повторная проверка вида ассета здесь лишь
// защита на случай рассинхронизации со списком допустимых видов.
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

	// Признак срочности не меняется после создания видео — читается текущим состоянием, чтобы
	// повторная публикация ушла в ту же полосу, что и исходная (§2 дизайна эпика Э5).
	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	assets, err := s.srv.VideoAsset.Get(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	original := findAssetByKind(assets, domain.VideoAssetKindOriginal)
	if original == nil {
		// Оригинал не может пропасть при штатной работе (BR-37, воркер его не трогает) — раз
		// это всё же произошло, повтор ничего не изменит: poison-случай (Д-3 ревью эпика).
		// Возврат ошибки здесь уронил бы всю транзакцию (включая уже применённый выше переход
		// в queued) и обработчик ушёл бы в бесконечный ретрай — вместо этого видео переводится
		// в failed(permanent) с тем attempt, что уже записан транзакцией (next), и событие
		// подтверждается (return nil).
		zap.L().Warn("temporary failure requeue is poison: original asset missing, failing video",
			zap.String("video_id", videoID.String()),
			zap.Int("attempt", next),
		)
		return s.failProcessing(ctx, videoID, next, domain.VideoFailureClassPermanent, "original asset missing")
	}

	return s.publishOriginalUploaded(ctx, videoID, next, video.IsUrgent, original.ObjectKey, s3.ObjectInfo{
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
	if err := s.canWatch(ctx, accountID, groupID, initiatorID); err != nil {
		return nil, err
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

	authorIDs := make([]uuid.UUID, len(videos))
	for i, video := range videos {
		authorIDs[i] = video.Author
	}

	authors, err := s.authorsByID(ctx, authorIDs)
	if err != nil {
		return nil, err
	}

	// Позиция в очереди на обработку (эпик Э5, §3 дизайна, В-3): один запрос на весь список
	// независимо от того, сколько видео в группе и сколько из них сейчас в очереди — без
	// round-trip'а на каждое видео. Позиция считается глобально по системе, поэтому результат
	// (по всем группам) мёржится в DTO только для элементов со статусом queued.
	queuePositions, err := s.repo.SelectQueuePositions(ctx)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	items := make([]domain.VideoListItem, len(videos))
	for i, video := range videos {
		items[i] = newVideoListItem(video, assetsByVideo[video.ID], canManage)
		items[i].Author = authorFor(video.Author, authors)

		if video.Status == domain.VideoStatusQueued {
			if position, ok := queuePositions[video.ID]; ok {
				position := position
				items[i].QueuePosition = &position
			}
		}
	}

	return items, nil
}

// authorsByID батчем резолвит пользователей по уникальным идентификаторам в карту id → User
// (П-6 контракта Э2, автор видео объектом). Дубликаты во входном списке схлопываются перед
// запросом; пустой список не порождает запроса к БД.
func (s *VideoService) authorsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.User, error) {
	unique := uniqueUUIDs(ids)

	users, err := s.srv.User.GetByIDs(ctx, unique)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	byID := make(map[uuid.UUID]domain.User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}

	return byID, nil
}

// authorFor собирает domain.VideoAuthor по id создателя видео и карте резолвнутых
// пользователей. Пользователь, которого не удалось найти, — не ошибка (П-6 контракта Э2):
// автор остаётся с заполненным только ID, без имени и фамилии.
func authorFor(id uuid.UUID, users map[uuid.UUID]domain.User) domain.VideoAuthor {
	if user, ok := users[id]; ok {
		return domain.VideoAuthor{ID: id, Name: user.Name, Surname: user.Surname}
	}

	return domain.VideoAuthor{ID: id}
}

// uniqueUUIDs возвращает список идентификаторов без повторов, сохраняя порядок первого
// появления.
func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	return unique
}

// videoListItemWithAuthor собирает карточку одного видео вида VideoListItem — того же, что и
// элемент списка видео (Э2-Т16, §5.1 контракта Э2): используется Rename и CompleteUpload, чтобы
// ответ был единообразен со списком. canManage передаётся вызывающей стороной без повторной
// проверки прав — оба метода уже требуют ManageVideo для самого вызова.
func (s *VideoService) videoListItemWithAuthor(
	ctx context.Context,
	video domain.Video,
	canManage bool,
) (domain.VideoListItem, error) {
	assets, err := s.srv.VideoAsset.Get(ctx, video.ID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoListItem{}, err
	}

	item := newVideoListItem(video, assets, canManage)

	authors, err := s.authorsByID(ctx, []uuid.UUID{video.Author})
	if err != nil {
		return domain.VideoListItem{}, err
	}
	item.Author = authorFor(video.Author, authors)

	return item, nil
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

// canWatch проверяет право инициатора на просмотр видео группы (Get, GetAll и список видео
// группы): аккаунтное или групповое VideoWatch, а также аккаунтное или групповое ManageVideo —
// право управления видео включает право его просмотра (решение ведущего по Д-6 ревью эпика Э2).
// Owner (аккаунта или группы) проходит проверку автоматически. Логика вынесена в
// Access.CanWatchVideo (§0 дизайна эпика Э3) — используется и для проверки доступа
// произвольного пользователя (не только инициатора запроса), например при назначении обучения.
func (s *VideoService) canWatch(ctx context.Context, accountID, groupID, initiatorID uuid.UUID) error {
	if s.srv.Access.CanWatchVideo(ctx, accountID, initiatorID, groupID) {
		return nil
	}

	return ErrForbidden
}

// canManageVideo определяет, доступно ли инициатору право ManageVideo — аккаунтное или
// групповое (OR-логика) — без возврата ошибки. Используется там, где отсутствие права не
// запрещает действие целиком, а лишь скрывает часть ответа (Э1-Т17: причина сбоя видна только
// с ManageVideo). Логика вынесена в Access.CanManageVideo (§2 дизайна эпика Э4) — тот же
// приём, каким эпик Э3 вынес CanWatchVideo.
func (s *VideoService) canManageVideo(ctx context.Context, accountID, groupID, initiatorID uuid.UUID) bool {
	return s.srv.Access.CanManageVideo(ctx, accountID, groupID, initiatorID) == nil
}

// Rename переименовывает видео и возвращает карточку того же вида, что и элемент списка
// видео (VideoListItem с профилями и автором, §5.1 контракта Э2) — чтобы фронту не нужно было
// перезапрашивать список ради актуального представления переименованного видео.
func (s *VideoService) Rename(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
	name string,
) (domain.VideoListItem, error) {
	// Право ManageVideo — аккаунтное или групповое (Access.CanManageVideo, §2 дизайна эпика Э4).
	if err := s.srv.Access.CanManageVideo(ctx, accountID, groupID, initiatorID); err != nil {
		return domain.VideoListItem{}, err
	}

	// Проверка, что видео принадлежит указанной группе (Б-1 ревью эпика: без неё право
	// ManageVideo в своей группе позволяло переименовать чужое видео — IDOR).
	if err := s.checkVideoBelongsToGroup(ctx, groupID, videoID); err != nil {
		return domain.VideoListItem{}, err
	}

	// Переименование видео
	video, err := s.repo.UpdateName(ctx, videoID, name)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoListItem{}, err
	}

	// Право ManageVideo уже подтверждено проверкой выше — повторно не проверяем.
	return s.videoListItemWithAuthor(ctx, video, true)
}

// checkVideoBelongsToGroup перечитывает видео и убеждается, что оно принадлежит groupID —
// защита от IDOR (Б-1 ревью эпика): проверка прав ManageVideo проходит по groupID из пути
// запроса, но без этой проверки videoID мог указывать на видео другой группы/аккаунта. Тот же
// код ответа, что и в Get: 404, если видео не найдено, 403 — если принадлежит другой группе.
func (s *VideoService) checkVideoBelongsToGroup(ctx context.Context, groupID, videoID uuid.UUID) error {
	video, err := s.repo.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if video.GroupID != groupID {
		zap.L().Error("video does not belong to the specified group")
		return ErrForbidden
	}

	return nil
}

func (s *VideoService) Delete(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
) error {
	// Право ManageVideo — аккаунтное или групповое (Access.CanManageVideo, §2 дизайна эпика Э4).
	if err := s.srv.Access.CanManageVideo(ctx, accountID, groupID, initiatorID); err != nil {
		return err
	}

	// Проверка, что видео принадлежит указанной группе (Б-1 ревью эпика: без неё право
	// ManageVideo в своей группе позволяло удалить чужое видео вместе с его объектами S3 — IDOR).
	if err := s.checkVideoBelongsToGroup(ctx, groupID, videoID); err != nil {
		return err
	}

	// Каскад обязательного обучения: назначения этого видео отменяются до удаления строки
	// видео (§4 «Каскады» дизайна эпика Э3, Э3-Т28) — после удаления связь уже потеряна
	// (FK ON DELETE SET NULL), а снимок названия остаётся в самом назначении.
	if err := s.srv.Assignment.OnVideoDeleted(ctx, videoID); err != nil {
		zap.L().Error(err.Error())
		return err
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

// videoLaneName возвращает человекочитаемое название полосы обработки для сообщений об ошибке.
func videoLaneName(isUrgent bool) string {
	if isUrgent {
		return "срочной"
	}
	return "архивной"
}

// failQueuedStalled переводит в failed(timeout) зависшие в queued видео полосы isUrgent, если
// индикатор её прогресса (§1 дизайна эпика Э5, исправление Д-1) не обновлялся дольше
// QueuedStallTimeout — то есть ни одно видео этой полосы не было реально взято в обработку за
// этот срок. Если полоса продвигается — второй проход для неё не выполняется вовсе, поэтому
// сколь угодно длинная, но честно продвигающаяся очередь ни одного видео не роняет.
func (s *VideoService) failQueuedStalled(ctx context.Context, now time.Time, isUrgent bool) ([]uuid.UUID, error) {
	progress, err := s.pipelineProgress.Select(ctx, isUrgent)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	stallDeadline := now.Add(-s.cfg.Video.QueuedStallTimeout)
	if progress.LastDequeuedAt.After(stallDeadline) {
		return nil, nil
	}

	return s.repo.UpdateTimedOut(
		ctx,
		domain.VideoStatusQueued,
		stallDeadline,
		domain.VideoFailure{
			Class: domain.VideoFailureClassTimeout,
			Reason: fmt.Sprintf(
				"конвейер %s полосы не продвигается уже %s", videoLaneName(isUrgent), s.cfg.Video.QueuedStallTimeout,
			),
		},
		&isUrgent,
	)
}

// FailTimedOut переводит в failed(timeout) видео, зависшие в uploading/queued/compressing
// дольше сконфигурированных таймаутов (§8 дизайна эпика Э1, §1 дизайна эпика Э5). Вызывается
// watchdog'ом на каждом тике. Каждый переход — один атомарный условный UPDATE
// (repository.Video.UpdateTimedOut), поэтому метод безопасен при нескольких одновременно
// работающих инстансах API: строку, которую уже перевёл другой инстанс, повторный UPDATE не
// затронет (WHERE status = <исходный статус> перестаёт совпадать) — гонки не возникает.
//
// Статус queued различает два порога (эпик Э5, исправление Д-1): безусловный
// QueuedMaxTimeout — потолок, не зависящий от активности полосы (Н4), и условный
// QueuedStallTimeout — срабатывает точечно для той полосы (архивной/срочной), чей индикатор
// прогресса (app.pipeline_progress) не продвигался дольше порога. Так длинная, но честно
// продвигающаяся очередь не роняется, а действительно остановившийся конвейер (например, все
// воркеры полосы упали) по-прежнему ловится без сколь угодно долгого ожидания.
func (s *VideoService) FailTimedOut(ctx context.Context, now time.Time) (domain.TimedOutReport, error) {
	uploading, err := s.repo.UpdateTimedOut(
		ctx,
		domain.VideoStatusUploading,
		now.Add(-s.cfg.Video.UploadTimeout),
		domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: fmt.Sprintf("загрузка не завершена за %s", s.cfg.Video.UploadTimeout),
		},
		nil,
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
		now.Add(-s.cfg.Video.QueuedMaxTimeout),
		domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: fmt.Sprintf("не взято в обработку за %s", s.cfg.Video.QueuedMaxTimeout),
		},
		nil,
	)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.TimedOutReport{}, err
	}

	for _, isUrgent := range []bool{false, true} {
		stalled, stallErr := s.failQueuedStalled(ctx, now, isUrgent)
		if stallErr != nil {
			return domain.TimedOutReport{}, stallErr
		}
		queued = append(queued, stalled...)
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
		nil,
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
