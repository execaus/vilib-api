package service

import (
	"context"
	"errors"
	"math"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// watchRateTolerance — допуск антиперемотки (В-1 решение владельца, Э3-Т14): длина
	// принятого интервала не может превышать реально прошедшее серверное время более чем на
	// 10%; тот же допуск используется для wall-cap (§3 шаг 5 дизайна эпика Э3).
	watchRateTolerance = 1.1
	// watchMaxPlaybackRate — интервалы со скоростью воспроизведения выше 1.0 отбрасываются
	// целиком (В-1 решение владельца: регулятор скорости в плеер не добавляется).
	watchMaxPlaybackRate = 1.0
	// watchIntervalDurationSlackMs — допуск превышения известной длительности видео при
	// валидации присланного интервала (§3 шаг 1 дизайна эпика Э3).
	watchIntervalDurationSlackMs = int64(1000)
	// watchPercentMultiplier — перевод доли покрытия/порога в проценты для ответа и БД.
	watchPercentMultiplier = 100
	// watchMaxPercent — верхняя граница процента покрытия в ответе (Э3-Т11).
	watchMaxPercent = 100

	// watchHeartbeatRejectedLog — имя структурированного лога отклонённых/усечённых
	// интервалов heartbeat'а (Э3-Н8).
	watchHeartbeatRejectedLog = "watch_heartbeat_rejected"

	watchRejectReasonRate    = "rate"
	watchRejectReasonTooLong = "too_long"
	watchRejectReasonWallCap = "wall_cap"
	watchRejectReasonForeign = "foreign_session"
)

// watchNeedMsUnknownDuration — сентинел needMs, когда длительность видео ещё не известна:
// порог недостижим, интервалы копятся до появления duration_ms и досчёта OnDurationKnown
// (§3 дизайна эпика Э3).
const watchNeedMsUnknownDuration = int64(math.MaxInt64)

// watchIntervalOutcome — результат оценки присланного интервала heartbeat'а (§3 шаги 4–5
// дизайна эпика Э3): applied решает, вызывать ли WatchProgress.Apply (интервал хоть частично
// зачтён) или только WatchProgress.UpdatePosition (интервал отброшен целиком).
type watchIntervalOutcome struct {
	finalToMs   int64
	wallDeltaMs int64
	applied     bool
	// reason — причина усечения/отклонения для лога Н8; пустая строка — интервал принят как
	// есть, логировать нечего.
	reason string
}

// WatchProgressService реализует приём heartbeat'ов плеера и алгоритм зачёта просмотра (§3
// дизайна эпика Э3).
type WatchProgressService struct {
	repo         repository.WatchProgress
	sessions     repository.WatchSession
	participants repository.AssignmentParticipant
	video        repository.Video
	srv          *Service
	cfg          config.VideoConfig
	// now — источник текущего времени; в проде time.Now, в тестах подменяется опцией
	// WithWatchProgressNow для детерминированных проверок усечения и wall-cap.
	now func() time.Time
}

// WatchProgressServiceOption настраивает WatchProgressService сверх обязательных зависимостей
// конструктора.
type WatchProgressServiceOption func(*WatchProgressService)

// WithWatchProgressNow подменяет источник текущего времени. Предназначена для тестов.
func WithWatchProgressNow(now func() time.Time) WatchProgressServiceOption {
	return func(s *WatchProgressService) {
		s.now = now
	}
}

func NewWatchProgressService(
	repo repository.WatchProgress,
	sessions repository.WatchSession,
	participants repository.AssignmentParticipant,
	video repository.Video,
	srv *Service,
	cfg config.VideoConfig,
	opts ...WatchProgressServiceOption,
) *WatchProgressService {
	s := &WatchProgressService{
		repo: repo, sessions: sessions, participants: participants, video: video, srv: srv, cfg: cfg,
		now: time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Heartbeat принимает отрезок непрерывного воспроизведения от плеера и обновляет прогресс
// просмотра инициатора по видео — шаги 1–8 алгоритма зачёта (§3 дизайна эпика Э3).
func (s *WatchProgressService) Heartbeat(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
	in domain.Heartbeat,
) (domain.WatchState, error) {
	now := s.now()

	// Шаг 1: права, состояние видео, валидация интервала.
	video, err := s.resolveWatchableVideo(ctx, accountID, groupID, initiatorID, videoID)
	if err != nil {
		return domain.WatchState{}, err
	}
	if video.Status == domain.VideoStatusUploading || video.Status == domain.VideoStatusFailed {
		return domain.WatchState{}, ErrVideoNotAvailable
	}
	if err = validateHeartbeatInterval(in, video.DurationMs); err != nil {
		return domain.WatchState{}, err
	}

	// Шаг 2: строка прогресса и сессия под замком (порядок блокировок фиксирован).
	progress, isNewProgress, err := s.lockProgress(ctx, initiatorID, videoID, now)
	if err != nil {
		return domain.WatchState{}, err
	}

	session, isNewSession, err := s.resolveSession(ctx, initiatorID, videoID, in, now)
	if err != nil {
		return domain.WatchState{}, err
	}

	// Шаг 3: чужая сессия и идемпотентность повтора.
	if !isNewSession {
		if session.UserID != initiatorID || session.VideoID != videoID {
			zap.L().Warn(watchHeartbeatRejectedLog,
				zap.String("reason", watchRejectReasonForeign),
				zap.String("session_id", in.SessionID.String()),
			)
			return domain.WatchState{}, ErrForbidden
		}
		if in.Seq <= session.LastSeq {
			return buildWatchState(progress, video.DurationMs, false), nil
		}
	}

	// Шаги 4–5: усечение перемотки и wall-cap.
	outcome := s.evaluateHeartbeatInterval(progress, isNewProgress, isNewSession, session.LastAt, in, now)
	logRejectedInterval(outcome, in)

	return s.finishHeartbeat(ctx, initiatorID, videoID, in, video, progress, outcome, now)
}

// resolveWatchableVideo проверяет право инициатора смотреть видео группы и то, что видео
// действительно принадлежит указанной группе (защита от IDOR, как VideoService.canWatch/
// checkVideoBelongsToGroup) — общая часть Heartbeat и Get.
func (s *WatchProgressService) resolveWatchableVideo(
	ctx context.Context,
	accountID, groupID, initiatorID, videoID uuid.UUID,
) (domain.Video, error) {
	if !s.srv.Access.CanWatchVideo(ctx, accountID, initiatorID, groupID) {
		return domain.Video{}, ErrForbidden
	}

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

// validateHeartbeatInterval проверяет базовую корректность присланного интервала (§3 шаг 1):
// to_ms не может быть меньше from_ms, а при известной длительности видео — превышать её более
// чем на watchIntervalDurationSlackMs.
func validateHeartbeatInterval(in domain.Heartbeat, durationMs *int64) error {
	if in.ToMs < in.FromMs {
		return ErrIntervalInvalid
	}
	if durationMs != nil && in.ToMs > *durationMs+watchIntervalDurationSlackMs {
		return ErrIntervalInvalid
	}
	return nil
}

// lockProgress выбирает и блокирует строку прогресса пользователя по видео, создавая пустую
// строку при первом обращении (§3 шаг 2). Возвращает признак того, что строка только что
// создана — нужен шагу 5 (wall-cap первого heartbeat'а).
func (s *WatchProgressService) lockProgress(
	ctx context.Context, userID, videoID uuid.UUID, now time.Time,
) (domain.WatchProgress, bool, error) {
	progress, err := s.repo.SelectForUpdate(ctx, userID, videoID)
	if errors.Is(err, repository.ErrNotFound) {
		if insertErr := s.repo.InsertEmpty(ctx, userID, videoID, now); insertErr != nil {
			zap.L().Error(insertErr.Error())
			return domain.WatchProgress{}, false, insertErr
		}

		progress, err = s.repo.SelectForUpdate(ctx, userID, videoID)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.WatchProgress{}, false, err
		}

		return progress, true, nil
	}
	if err != nil {
		zap.L().Error(err.Error())
		return domain.WatchProgress{}, false, err
	}

	return progress, false, nil
}

// resolveSession выбирает и блокирует сессию по идентификатору, создавая новую при первом
// обращении (§3 шаг 3). Возвращает признак того, что сессия только что создана — нужен шагу 4
// (базовый elapsed первого heartbeat'а сессии).
func (s *WatchProgressService) resolveSession(
	ctx context.Context, userID, videoID uuid.UUID, in domain.Heartbeat, now time.Time,
) (domain.WatchSession, bool, error) {
	session, err := s.sessions.SelectForUpdate(ctx, in.SessionID)
	if errors.Is(err, repository.ErrNotFound) {
		session, err = s.sessions.Insert(ctx, in.SessionID, userID, videoID, now, in.PositionMs)
		if err != nil {
			zap.L().Error(err.Error())
			return domain.WatchSession{}, false, err
		}

		return session, true, nil
	}
	if err != nil {
		zap.L().Error(err.Error())
		return domain.WatchSession{}, false, err
	}

	return session, false, nil
}

// evaluateHeartbeatInterval реализует шаги 4–5 алгоритма зачёта: отбрасывание интервалов со
// скоростью выше 1.0, усечение перемотки (elapsed × 1.1) и wall-cap (покрытие не может расти
// быстрее времени у стены × 1.1).
func (s *WatchProgressService) evaluateHeartbeatInterval(
	progress domain.WatchProgress, isNewProgress bool,
	isNewSession bool, sessionLastAt time.Time,
	in domain.Heartbeat, now time.Time,
) watchIntervalOutcome {
	claimedLenMs := in.ToMs - in.FromMs

	if in.PlaybackRate > watchMaxPlaybackRate {
		return watchIntervalOutcome{reason: watchRejectReasonRate}
	}
	if claimedLenMs == 0 {
		// Нет воспроизведения (пауза/seek) — не отклонение, просто нечего засчитывать.
		return watchIntervalOutcome{}
	}

	elapsed := s.cfg.WatchHeartbeatInterval
	if !isNewSession {
		elapsed = now.Sub(sessionLastAt)
	}
	if elapsed < 0 {
		elapsed = 0
	}

	truncatedToMs := in.ToMs
	reason := ""
	maxLenMs := int64(float64(elapsed.Milliseconds()) * watchRateTolerance)
	if claimedLenMs > maxLenMs {
		truncatedToMs = in.FromMs + maxLenMs
		reason = watchRejectReasonTooLong
	}

	acceptedLenMs := truncatedToMs - in.FromMs
	if acceptedLenMs <= 0 {
		return watchIntervalOutcome{reason: reason}
	}

	wallDeltaMs := s.watchWallDelta(progress, isNewProgress, acceptedLenMs, now)
	allowanceMs := int64(float64(progress.WallMs+wallDeltaMs)*watchRateTolerance) - progress.CoveredMs
	if allowanceMs <= 0 {
		return watchIntervalOutcome{reason: watchRejectReasonWallCap}
	}

	if allowanceMs < acceptedLenMs {
		truncatedToMs = in.FromMs + allowanceMs
		reason = watchRejectReasonWallCap
	}

	return watchIntervalOutcome{finalToMs: truncatedToMs, wallDeltaMs: wallDeltaMs, applied: true, reason: reason}
}

// watchWallDelta вычисляет приращение «времени у стены» (§3 шаг 5): для новой строки прогресса
// ограничивается конфигурационным интервалом heartbeat'а × 1.1 (нет предыдущего last_at, из
// которого считать), иначе — реально прошедшим временем с предыдущего heartbeat'а.
func (s *WatchProgressService) watchWallDelta(
	progress domain.WatchProgress, isNewProgress bool, acceptedLenMs int64, now time.Time,
) int64 {
	if isNewProgress {
		capMs := int64(float64(s.cfg.WatchHeartbeatInterval.Milliseconds()) * watchRateTolerance)
		return min(acceptedLenMs, capMs)
	}

	sinceLastMs := max(now.Sub(progress.LastAt).Milliseconds(), 0)

	return min(acceptedLenMs, sinceLastMs)
}

// finishHeartbeat выполняет шаги 6–8: единственный UPDATE применения интервала (или только
// позиции при полном отклонении), продвижение сессии, переходы статусов участников и сборку
// ответа.
func (s *WatchProgressService) finishHeartbeat(
	ctx context.Context,
	userID, videoID uuid.UUID,
	in domain.Heartbeat,
	video domain.Video,
	progressBefore domain.WatchProgress,
	outcome watchIntervalOutcome,
	now time.Time,
) (domain.WatchState, error) {
	var (
		newProgress domain.WatchProgress
		err         error
	)

	if outcome.applied {
		newProgress, err = s.repo.Apply(
			ctx, userID, videoID,
			in.FromMs, outcome.finalToMs, in.PositionMs, outcome.wallDeltaMs,
			now, s.watchNeedMs(video.DurationMs),
		)
	} else {
		newProgress, err = s.repo.UpdatePosition(ctx, userID, videoID, in.PositionMs, now)
	}
	if err != nil {
		zap.L().Error(err.Error())
		return domain.WatchState{}, err
	}

	if _, err = s.sessions.Update(ctx, in.SessionID, in.Seq, now, in.PositionMs); err != nil {
		zap.L().Error(err.Error())
		return domain.WatchState{}, err
	}

	if outcome.applied {
		if err = s.applyParticipantTransitions(
			ctx, userID, videoID, progressBefore, newProgress, video, in.SessionID, now,
		); err != nil {
			return domain.WatchState{}, err
		}
	}

	return buildWatchState(newProgress, video.DurationMs, outcome.applied), nil
}

// applyParticipantTransitions реализует шаг 7: assigned→in_progress при первом принятом
// интервале и завершение участия при первом достижении порога (§3 дизайна эпика Э3).
func (s *WatchProgressService) applyParticipantTransitions(
	ctx context.Context,
	userID, videoID uuid.UUID,
	before, after domain.WatchProgress,
	video domain.Video,
	sessionID uuid.UUID,
	now time.Time,
) error {
	if before.CoveredMs == 0 && after.CoveredMs > 0 {
		if _, err := s.participants.UpdateStatusByUserVideo(
			ctx, userID, videoID,
			domain.AssignmentParticipantStatusAssigned, domain.AssignmentParticipantStatusInProgress,
		); err != nil {
			zap.L().Error(err.Error())
			return err
		}
	}

	firstCrossing := before.ThresholdReachedAt == nil && after.ThresholdReachedAt != nil
	if video.DurationMs == nil || !firstCrossing {
		return nil
	}

	coveragePct := coveragePercent(after.CoveredMs, video.DurationMs)
	session := sessionID

	if _, err := s.participants.CompleteByUserVideo(
		ctx, userID, videoID, now, coveragePct, s.thresholdPercent(), &session,
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// Get возвращает текущее состояние прогресса просмотра инициатора по видео без изменений —
// нет строки прогресса означает, что просмотр ещё не начинался (нулевое состояние).
func (s *WatchProgressService) Get(
	ctx context.Context, accountID, groupID, initiatorID, videoID uuid.UUID,
) (domain.WatchState, error) {
	video, err := s.resolveWatchableVideo(ctx, accountID, groupID, initiatorID, videoID)
	if err != nil {
		return domain.WatchState{}, err
	}

	progresses, err := s.repo.SelectByUserAndVideoIDs(ctx, initiatorID, []uuid.UUID{videoID})
	if err != nil {
		zap.L().Error(err.Error())
		return domain.WatchState{}, err
	}
	if len(progresses) == 0 {
		return domain.WatchState{DurationMs: video.DurationMs}, nil
	}

	return buildWatchState(progresses[0], video.DurationMs, false), nil
}

// OnDurationKnown досчитывает зачёт для пользователей, чей прогресс уже достиг порога до того,
// как стала известна длительность видео (§3, «Э3-Т6») — вызывается из
// VideoService.ApplyProcessingCompleted при переходе видео в ready. Идемпотентна: повторный
// вызов не находит новых кандидатов (threshold_reached_at уже проставлен).
func (s *WatchProgressService) OnDurationKnown(ctx context.Context, videoID uuid.UUID, durationMs int64) error {
	if durationMs <= 0 {
		return nil
	}

	needMs := s.watchNeedMs(&durationMs)
	now := s.now()

	userIDs, err := s.repo.OnDurationKnown(ctx, videoID, needMs, now)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	for _, userID := range userIDs {
		if err = s.completeAfterDurationKnown(ctx, userID, videoID, durationMs, now); err != nil {
			return err
		}
	}

	return nil
}

// completeAfterDurationKnown завершает участие одного пользователя, найденного
// WatchProgress.OnDurationKnown — вынесено отдельно, чтобы цикл по пользователям оставался
// коротким.
func (s *WatchProgressService) completeAfterDurationKnown(
	ctx context.Context, userID, videoID uuid.UUID, durationMs int64, now time.Time,
) error {
	progresses, err := s.repo.SelectByUserAndVideoIDs(ctx, userID, []uuid.UUID{videoID})
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	coveragePct := 0
	if len(progresses) > 0 {
		coveragePct = coveragePercent(progresses[0].CoveredMs, &durationMs)
	}

	if _, err = s.participants.CompleteByUserVideo(
		ctx, userID, videoID, now, coveragePct, s.thresholdPercent(), nil,
	); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// watchNeedMs переводит долю порога зачёта в миллисекунды покрытия, нужные для его достижения;
// nil (длительность видео ещё не известна) — сентинел watchNeedMsUnknownDuration.
func (s *WatchProgressService) watchNeedMs(durationMs *int64) int64 {
	if durationMs == nil {
		return watchNeedMsUnknownDuration
	}

	return int64(math.Ceil(s.cfg.WatchCompletionThreshold * float64(*durationMs)))
}

// thresholdPercent переводит долю порога зачёта в целый процент — версия правила, сохраняемая
// в completed_threshold_pct (Э3-Н1).
func (s *WatchProgressService) thresholdPercent() int {
	return int(math.Round(s.cfg.WatchCompletionThreshold * watchPercentMultiplier))
}

// coveragePercent вычисляет процент покрытия видео (floor, ограничен сверху watchMaxPercent).
// Длительность видео неизвестна или некорректна — 0 (Э3-Т6: до появления duration_ms прогресс
// копится без процента).
func coveragePercent(coveredMs int64, durationMs *int64) int {
	if durationMs == nil || *durationMs <= 0 {
		return 0
	}

	pct := max(min(coveredMs*watchPercentMultiplier / *durationMs, watchMaxPercent), 0)

	return int(pct)
}

// buildWatchState собирает ответ WatchState из строки прогресса (§3 дизайна эпика Э3).
func buildWatchState(progress domain.WatchProgress, durationMs *int64, accepted bool) domain.WatchState {
	return domain.WatchState{
		CoveredMs:      progress.CoveredMs,
		CoveragePct:    coveragePercent(progress.CoveredMs, durationMs),
		Completed:      progress.ThresholdReachedAt != nil,
		LastPositionMs: progress.LastPositionMs,
		DurationMs:     durationMs,
		Accepted:       accepted,
	}
}

// logRejectedInterval пишет структурированный лог отклонённого/усечённого интервала (Э3-Н8).
// Полностью принятый интервал (reason == "") лога не оставляет.
func logRejectedInterval(outcome watchIntervalOutcome, in domain.Heartbeat) {
	if outcome.reason == "" {
		return
	}

	acceptedMs := int64(0)
	if outcome.applied {
		acceptedMs = outcome.finalToMs - in.FromMs
	}

	zap.L().Info(watchHeartbeatRejectedLog,
		zap.String("reason", outcome.reason),
		zap.Int64("claimed_ms", in.ToMs-in.FromMs),
		zap.Int64("accepted_ms", acceptedMs),
	)
}
