package service

import (
	"context"
	"errors"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// chapterLimitPerVideo — максимальное число глав на одно видео (Э4-Т3).
const chapterLimitPerVideo = 100

// ChapterService реализует CRUD глав видео и выдачу глав с покрытием просмотра (§4 дизайна
// эпика Э4): границы и покрытие глав считаются в SQL репозитория (LEAD/COALESCE, пересечение
// intervals с int8multirange), сервис — только права, IDOR и бизнес-валидация.
type ChapterService struct {
	repo  repository.Chapter
	video repository.Video
	srv   *Service
	cfg   config.VideoConfig
}

// NewChapterService создаёт ChapterService. video читается напрямую из репозитория видео
// (в обход VideoService) — тот же приём, каким AssignmentService и WatchProgressService читают
// видео при валидации (§4 дизайна эпика Э4).
func NewChapterService(
	repo repository.Chapter,
	video repository.Video,
	srv *Service,
	cfg config.VideoConfig,
) *ChapterService {
	return &ChapterService{repo: repo, video: video, srv: srv, cfg: cfg}
}

// List возвращает главы видео, упорядоченные по времени начала, вместе с покрытием
// инициатора (§4 дизайна эпика Э4, шаги 1–3). Видео без глав — пустой список, не ошибка
// (Э4-Т4): проверка статуса ready намеренно не выполняется — главы просмотра здесь и не может
// быть, если видео ни разу не было готово.
func (s *ChapterService) List(
	ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID,
) ([]domain.ChapterProgress, error) {
	if !s.srv.Access.CanWatchVideo(ctx, accountID, initiatorID, groupID) {
		return nil, ErrForbidden
	}

	video, err := s.checkVideoBelongsToGroup(ctx, groupID, videoID)
	if err != nil {
		return nil, err
	}

	progress, err := s.repo.SelectProgressByVideoAndUser(ctx, videoID, initiatorID, chapterDurationOrZero(video))
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return progress, nil
}

// Create создаёт главу видео (§4 дизайна эпика Э4, шаги 1–6): право ManageVideo, принадлежность
// видео группе (IDOR), готовность видео, диапазон начала, лимит числа глав, конфликт по
// уникальности начала. Ответ собирается пересчётом границ всех глав видео, чтобы вернуть
// вычисленный EndMs созданной главы.
func (s *ChapterService) Create(
	ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID, in domain.CreateChapter,
) (domain.ChapterBound, error) {
	if err := s.srv.Access.CanManageVideo(ctx, accountID, groupID, initiatorID); err != nil {
		return domain.ChapterBound{}, err
	}

	video, err := s.checkVideoBelongsToGroup(ctx, groupID, videoID)
	if err != nil {
		return domain.ChapterBound{}, err
	}

	if video.Status != domain.VideoStatusReady || video.DurationMs == nil {
		return domain.ChapterBound{}, ErrVideoNotReadyForChapters
	}

	if in.StartMs < 0 || in.StartMs >= *video.DurationMs {
		return domain.ChapterBound{}, ErrChapterStartInvalid
	}

	count, err := s.repo.CountByVideoID(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.ChapterBound{}, err
	}
	if count >= chapterLimitPerVideo {
		return domain.ChapterBound{}, ErrChaptersLimit
	}

	chapter, err := s.repo.Insert(ctx, videoID, in.StartMs, in.Name)
	if err != nil {
		if errors.Is(dberrors.VideoChapterErrors.ErrUniqueVideoChaptersVideoIdStartMsKey, err) {
			zap.L().Warn(err.Error())
			return domain.ChapterBound{}, ErrChapterStartTaken
		}
		zap.L().Error(err.Error())
		return domain.ChapterBound{}, err
	}

	return s.boundAfterMutation(ctx, videoID, *video.DurationMs, chapter.ID)
}

// Update меняет начало и/или название главы (§4 дизайна эпика Э4): диапазон начала и
// готовность видео проверяются только при изменении StartMs — переименование без сдвига
// границы не требует повторной проверки (Э4-Т6, Э4-Т7).
func (s *ChapterService) Update(
	ctx context.Context, accountID, groupID, initiatorID, videoID, chapterID uuid.UUID, patch domain.ChapterPatch,
) (domain.ChapterBound, error) {
	if err := s.srv.Access.CanManageVideo(ctx, accountID, groupID, initiatorID); err != nil {
		return domain.ChapterBound{}, err
	}

	video, err := s.checkVideoBelongsToGroup(ctx, groupID, videoID)
	if err != nil {
		return domain.ChapterBound{}, err
	}

	if err = s.checkChapterBelongsToVideo(ctx, videoID, chapterID); err != nil {
		return domain.ChapterBound{}, err
	}

	if patch.StartMs != nil {
		if video.Status != domain.VideoStatusReady || video.DurationMs == nil {
			return domain.ChapterBound{}, ErrVideoNotReadyForChapters
		}
		if *patch.StartMs < 0 || *patch.StartMs >= *video.DurationMs {
			return domain.ChapterBound{}, ErrChapterStartInvalid
		}
	}

	if video.DurationMs == nil {
		// Глава уже существует, значит видео было готово на момент её создания (главы
		// заводятся только у ready-видео) и длительность назад не сбрасывается (§0 фактов
		// дизайна эпика Э4) — это защитный случай, а не ожидаемый путь.
		zap.L().Error("chapter update: video duration is unknown for a video with existing chapters")
		return domain.ChapterBound{}, ErrVideoNotReadyForChapters
	}

	if _, err = s.repo.Update(ctx, chapterID, patch); err != nil {
		if errors.Is(dberrors.VideoChapterErrors.ErrUniqueVideoChaptersVideoIdStartMsKey, err) {
			zap.L().Warn(err.Error())
			return domain.ChapterBound{}, ErrChapterStartTaken
		}
		zap.L().Error(err.Error())
		return domain.ChapterBound{}, err
	}

	return s.boundAfterMutation(ctx, videoID, *video.DurationMs, chapterID)
}

// Delete удаляет главу видео (§4 дизайна эпика Э4): соседние главы автоматически «наследуют»
// освободившийся промежуток при следующем чтении границ — дополнительного шага не требуется.
func (s *ChapterService) Delete(
	ctx context.Context, accountID, groupID, initiatorID, videoID, chapterID uuid.UUID,
) error {
	if err := s.srv.Access.CanManageVideo(ctx, accountID, groupID, initiatorID); err != nil {
		return err
	}

	if _, err := s.checkVideoBelongsToGroup(ctx, groupID, videoID); err != nil {
		return err
	}

	if err := s.checkChapterBelongsToVideo(ctx, videoID, chapterID); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, chapterID); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// checkVideoBelongsToGroup перечитывает видео и убеждается, что оно принадлежит groupID —
// защита от IDOR (тот же приём, что у VideoService.checkVideoBelongsToGroup, Б-1 ревью эпика
// Э2): 404, если видео не найдено, 403 — если принадлежит другой группе.
func (s *ChapterService) checkVideoBelongsToGroup(
	ctx context.Context,
	groupID, videoID uuid.UUID,
) (domain.Video, error) {
	video, err := s.video.Select(ctx, videoID)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	if video.GroupID != groupID {
		zap.L().Error("video does not belong to the specified group")
		return domain.Video{}, ErrForbidden
	}

	return *video, nil
}

// checkChapterBelongsToVideo перечитывает главу и убеждается, что она принадлежит videoID —
// защита от IDOR на уровне главы (иначе право ManageVideo в своей группе позволяло бы менять
// главу видео другой группы): строка не найдена — ErrNotFound, принадлежит другому видео —
// ErrForbidden.
func (s *ChapterService) checkChapterBelongsToVideo(ctx context.Context, videoID, chapterID uuid.UUID) error {
	chapter, err := s.repo.SelectByID(ctx, chapterID)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if chapter.VideoID != videoID {
		zap.L().Error("chapter does not belong to the specified video")
		return ErrForbidden
	}

	return nil
}

// boundAfterMutation пересчитывает границы всех глав видео и возвращает границу главы
// chapterID — единственный способ узнать вычисленный EndMs после Insert/Update (§4 дизайна
// эпика Э4).
func (s *ChapterService) boundAfterMutation(
	ctx context.Context, videoID uuid.UUID, durationMs int64, chapterID uuid.UUID,
) (domain.ChapterBound, error) {
	bounds, err := s.repo.SelectBoundsByVideoID(ctx, videoID, durationMs)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.ChapterBound{}, err
	}

	for _, bound := range bounds {
		if bound.ID == chapterID {
			return bound, nil
		}
	}

	zap.L().Error("mutated chapter is missing from bounds selection")
	return domain.ChapterBound{}, ErrNotFound
}

// chapterDurationOrZero возвращает длительность видео либо ноль, если она ещё не известна.
// Безопасно для List: без глав/без известной длительности запрос вернёт пустой список, значение
// параметра $duration_ms роли не играет (Э4-Т4).
func chapterDurationOrZero(video domain.Video) int64 {
	if video.DurationMs == nil {
		return 0
	}

	return *video.DurationMs
}
