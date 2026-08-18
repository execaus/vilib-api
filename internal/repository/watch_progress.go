package repository

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"
	"go.uber.org/zap"
)

// watchProgressPKConstraint — имя ограничения первичного ключа (user_id, video_id),
// сгенерированное Postgres по умолчанию (миграция не задаёт имя явно).
const watchProgressPKConstraint = "watch_progress_pkey"

type WatchProgressRepository struct {
	provider *ExecutorProvider
}

func NewWatchProgressRepository(provider *ExecutorProvider) *WatchProgressRepository {
	return &WatchProgressRepository{provider: provider}
}

// SelectForUpdate выбирает и блокирует (FOR UPDATE) строку прогресса пользователя по видео.
func (r *WatchProgressRepository) SelectForUpdate(
	ctx context.Context, userID, videoID uuid.UUID,
) (domain.WatchProgress, error) {
	exec := r.provider.GetExecutor(ctx)

	row, err := schema.WatchProgresses.Query(
		sm.Where(schema.WatchProgresses.Columns.UserID.EQ(psql.Arg(userID))),
		sm.Where(schema.WatchProgresses.Columns.VideoID.EQ(psql.Arg(videoID))),
		sm.ForUpdate(),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.WatchProgress{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.WatchProgress{}, err
	}

	var progress domain.WatchProgress
	progress.FromDB(row)

	return progress, nil
}

// InsertEmpty создаёт пустую строку прогресса (first_at/last_at = now) — идемпотентно, ON
// CONFLICT по первичному ключу ничего не делает, если строка уже существует. Вызывающая
// сторона обязана перечитать строку через SelectForUpdate, чтобы получить блокировку.
func (r *WatchProgressRepository) InsertEmpty(ctx context.Context, userID, videoID uuid.UUID, now time.Time) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.WatchProgresses.Insert(
		&schema.WatchProgressSetter{
			UserID:  omit.From(userID),
			VideoID: omit.From(videoID),
			FirstAt: omit.From(now),
			LastAt:  omit.From(now),
		},
		im.OnConflictOnConstraint(watchProgressPKConstraint).DoNothing(),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			// ON CONFLICT DO NOTHING не вернул строку — она уже существовала, это не ошибка.
			return nil
		}
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// watchProgressApplySQL — единственный UPDATE применения принятого интервала heartbeat'а
// (§3 шаг 6 дизайна эпика Э3): объединяет intervals оператором `+` для int8multirange,
// пересчитывает covered_ms без чтения-изменения в Go (unnest + sum), увеличивает wall_ms,
// обновляет last_position_ms/last_at и выставляет threshold_reached_at при первом достижении
// needMs — coalesce не даёт переписать уже зафиксированный момент повторными heartbeat'ами.
const watchProgressApplySQL = `
WITH updated AS (
	SELECT intervals + int8multirange(int8range(?::bigint, ?::bigint)) AS new_intervals
	FROM watch_progress
	WHERE user_id = ?::uuid AND video_id = ?::uuid
), covered AS (
	SELECT coalesce(sum(upper(r) - lower(r)), 0)::bigint AS covered_ms
	FROM updated, unnest(updated.new_intervals) AS r
)
UPDATE watch_progress
SET intervals = updated.new_intervals,
	covered_ms = covered.covered_ms,
	wall_ms = watch_progress.wall_ms + ?::bigint,
	last_position_ms = ?::bigint,
	last_at = ?::timestamp,
	threshold_reached_at = coalesce(
		watch_progress.threshold_reached_at,
		CASE WHEN covered.covered_ms >= ?::bigint THEN ?::timestamp END
	)
FROM updated, covered
WHERE watch_progress.user_id = ?::uuid AND watch_progress.video_id = ?::uuid
RETURNING watch_progress.user_id, watch_progress.video_id, watch_progress.intervals,
	watch_progress.covered_ms, watch_progress.last_position_ms, watch_progress.wall_ms,
	watch_progress.first_at, watch_progress.last_at, watch_progress.threshold_reached_at
`

// Apply — см. WatchProgress.Apply. Строка должна существовать и быть заблокирована
// предшествующим SelectForUpdate в той же транзакции.
func (r *WatchProgressRepository) Apply(
	ctx context.Context,
	userID, videoID uuid.UUID,
	fromMs, toMs, positionMs, wallDeltaMs int64,
	now time.Time, needMs int64,
) (domain.WatchProgress, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(
		watchProgressApplySQL,
		fromMs, toMs, userID, videoID,
		wallDeltaMs, positionMs, now, needMs, now,
		userID, videoID,
	)

	row, err := bob.One(ctx, exec, query, scan.StructMapper[*schema.WatchProgress]())
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.WatchProgress{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.WatchProgress{}, err
	}

	var progress domain.WatchProgress
	progress.FromDB(row)

	return progress, nil
}

// UpdatePosition обновляет только позицию плеера и last_at — heartbeat, чей интервал не был
// зачтён целиком (отброшен или урезан до нуля).
func (r *WatchProgressRepository) UpdatePosition(
	ctx context.Context, userID, videoID uuid.UUID, positionMs int64, now time.Time,
) (domain.WatchProgress, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.WatchProgressSetter{
		LastPositionMS: omit.From(positionMs),
		LastAt:         omit.From(now),
	}

	row, err := schema.WatchProgresses.Update(
		setter.UpdateMod(),
		um.Where(schema.WatchProgresses.Columns.UserID.EQ(psql.Arg(userID))),
		um.Where(schema.WatchProgresses.Columns.VideoID.EQ(psql.Arg(videoID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.WatchProgress{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.WatchProgress{}, err
	}

	var progress domain.WatchProgress
	progress.FromDB(row)

	return progress, nil
}

// OnDurationKnown выставляет threshold_reached_at всем строкам видео, чьё покрытие уже
// достигло needMs, но порог ещё не был зафиксирован (§3, «Э3-Т6»).
func (r *WatchProgressRepository) OnDurationKnown(
	ctx context.Context, videoID uuid.UUID, needMs int64, now time.Time,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	rows, err := schema.WatchProgresses.Update(
		(&schema.WatchProgressSetter{
			ThresholdReachedAt: omitnull.From(now),
		}).UpdateMod(),
		um.Where(schema.WatchProgresses.Columns.VideoID.EQ(psql.Arg(videoID))),
		um.Where(schema.WatchProgresses.Columns.ThresholdReachedAt.IsNull()),
		um.Where(schema.WatchProgresses.Columns.CoveredMS.GTE(psql.Arg(needMs))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	userIDs := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		userIDs[i] = row.UserID
	}

	return userIDs, nil
}

// SelectByUserAndVideoIDs батчем выбирает прогресс пользователя по списку видео. Пустой
// список идентификаторов не порождает запроса к БД.
func (r *WatchProgressRepository) SelectByUserAndVideoIDs(
	ctx context.Context, userID uuid.UUID, videoIDs []uuid.UUID,
) ([]domain.WatchProgress, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	videoIDArgs := make([]bob.Expression, len(videoIDs))
	for i, id := range videoIDs {
		videoIDArgs[i] = psql.Arg(id)
	}

	rows, err := schema.WatchProgresses.Query(
		sm.Where(schema.WatchProgresses.Columns.UserID.EQ(psql.Arg(userID))),
		sm.Where(schema.WatchProgresses.Columns.VideoID.In(videoIDArgs...)),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	progresses := make([]domain.WatchProgress, len(rows))
	for i, row := range rows {
		progresses[i].FromDB(row)
	}

	return progresses, nil
}

// SelectByVideoIDs батчем выбирает прогресс всех пользователей по списку видео (отчёты).
// Пустой список идентификаторов не порождает запроса к БД.
func (r *WatchProgressRepository) SelectByVideoIDs(
	ctx context.Context, videoIDs []uuid.UUID,
) ([]domain.WatchProgress, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	videoIDArgs := make([]bob.Expression, len(videoIDs))
	for i, id := range videoIDs {
		videoIDArgs[i] = psql.Arg(id)
	}

	rows, err := schema.WatchProgresses.Query(
		sm.Where(schema.WatchProgresses.Columns.VideoID.In(videoIDArgs...)),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	progresses := make([]domain.WatchProgress, len(rows))
	for i, row := range rows {
		progresses[i].FromDB(row)
	}

	return progresses, nil
}
