package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/scan"
	"go.uber.org/zap"
)

// chapterBoundsCTE — общий CTE границ глав видео (§3 дизайна эпика Э4): конец главы не
// хранится, а вычисляется как начало следующей главы того же видео по порядку start_ms
// (LEAD) либо переданная длительность видео (COALESCE) для последней главы. Собран как
// текстовый шаблон, а не отдельный запрос, чтобы участвовать одним round-trip'ом в каждом из
// трёх запросов ниже (Н1).
const chapterBoundsCTE = `
WITH chapter_bounds AS (
	SELECT chapter_id, video_id, name, start_ms, created_at,
		coalesce(lead(start_ms) OVER (PARTITION BY video_id ORDER BY start_ms), ?::bigint) AS end_ms
	FROM video_chapters
	WHERE video_id = ?::uuid
)
`

// chapterBoundRow — строка сканирования результата chapterBoundsSQL.
type chapterBoundRow struct {
	ChapterID uuid.UUID `db:"chapter_id"`
	VideoID   uuid.UUID `db:"video_id"`
	Name      string    `db:"name"`
	StartMs   int64     `db:"start_ms"`
	CreatedAt time.Time `db:"created_at"`
	EndMs     int64     `db:"end_ms"`
}

func (row chapterBoundRow) toDomain() domain.ChapterBound {
	return domain.ChapterBound{
		Chapter: domain.Chapter{
			ID:        row.ChapterID,
			VideoID:   row.VideoID,
			Name:      row.Name,
			StartMs:   row.StartMs,
			CreatedAt: row.CreatedAt,
		},
		EndMs: row.EndMs,
	}
}

// chapterBoundsSQL — границы всех глав видео без привязки к пользователю (§3 дизайна эпика
// Э4, первый запрос) — используется редактором и для сборки ответа после создания/правки главы.
const chapterBoundsSQL = chapterBoundsCTE + `
SELECT chapter_id, video_id, name, start_ms, created_at, end_ms
FROM chapter_bounds
ORDER BY start_ms
`

// chapterProgressSQL — границы глав видео с покрытием одного пользователя (§3 дизайна эпика
// Э4, второй запрос): пересечение watch_progress.intervals пользователя с границами каждой
// главы одним запросом, без чтения интервалов в приложение (Н1).
const chapterProgressSQL = chapterBoundsCTE + `
SELECT b.chapter_id, b.video_id, b.name, b.start_ms, b.created_at, b.end_ms,
	coalesce(cov.covered_ms, 0) AS covered_ms
FROM chapter_bounds b
LEFT JOIN LATERAL (
	SELECT coalesce(sum(upper(r) - lower(r)), 0)::bigint AS covered_ms
	FROM watch_progress wp,
		LATERAL unnest(wp.intervals * int8multirange(int8range(b.start_ms, b.end_ms))) r
	WHERE wp.user_id = ?::uuid AND wp.video_id = ?::uuid
) cov ON true
ORDER BY b.start_ms
`

// chapterUserProgressRow — строка сканирования результата chapterUserProgressSQLTemplate.
type chapterUserProgressRow struct {
	UserID    uuid.UUID `db:"user_id"`
	ChapterID uuid.UUID `db:"chapter_id"`
	VideoID   uuid.UUID `db:"video_id"`
	Name      string    `db:"name"`
	StartMs   int64     `db:"start_ms"`
	CreatedAt time.Time `db:"created_at"`
	EndMs     int64     `db:"end_ms"`
	CoveredMs int64     `db:"covered_ms"`
}

// chapterUserProgressSQLTemplate — границы глав видео с покрытием сразу многих пользователей
// (§3 дизайна эпика Э4, третий запрос, «многие пользователи сразу»): один запрос на всю
// карточку отчёта независимо от числа участников — не один запрос на участника (Н1). %s —
// список плейсхолдеров идентификаторов пользователей (тот же приём, что у
// countByAssignmentIDsSQLTemplate, эпик Э3).
const chapterUserProgressSQLTemplate = chapterBoundsCTE + `
SELECT wp.user_id, b.chapter_id, b.video_id, b.name, b.start_ms, b.created_at, b.end_ms,
	coalesce(sum(upper(r) - lower(r)), 0)::bigint AS covered_ms
FROM chapter_bounds b
JOIN watch_progress wp ON wp.video_id = b.video_id AND wp.user_id IN (%s)
LEFT JOIN LATERAL unnest(wp.intervals * int8multirange(int8range(b.start_ms, b.end_ms))) r ON true
GROUP BY wp.user_id, b.chapter_id, b.video_id, b.name, b.start_ms, b.created_at, b.end_ms
ORDER BY wp.user_id, b.start_ms
`

type ChapterRepository struct {
	provider *ExecutorProvider
}

func NewChapterRepository(provider *ExecutorProvider) *ChapterRepository {
	return &ChapterRepository{provider: provider}
}

// Insert создаёт главу видео. Конфликт по UNIQUE(video_id, start_ms) не перехватывается здесь —
// вызывающий сервис распознаёт его через dberrors.VideoChapterErrors и превращает в
// ErrChapterStartTaken (§4 дизайна эпика Э4).
func (r *ChapterRepository) Insert(ctx context.Context, videoID uuid.UUID, startMs int64, name string) (
	domain.Chapter, error,
) {
	exec := r.provider.GetExecutor(ctx)

	chapterDB, err := schema.VideoChapters.Insert(&schema.VideoChapterSetter{
		VideoID: omit.From(videoID),
		StartMS: omit.From(startMs),
		Name:    omit.From(name),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Chapter{}, err
	}

	var chapter domain.Chapter
	chapter.FromDB(chapterDB)

	return chapter, nil
}

// SelectByID выбирает главу по идентификатору. Строка не найдена — ErrNotFound.
func (r *ChapterRepository) SelectByID(ctx context.Context, chapterID uuid.UUID) (domain.Chapter, error) {
	exec := r.provider.GetExecutor(ctx)

	chapterDB, err := schema.VideoChapters.Query(
		sm.Where(schema.VideoChapters.Columns.ChapterID.EQ(psql.Arg(chapterID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Chapter{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Chapter{}, err
	}

	var chapter domain.Chapter
	chapter.FromDB(chapterDB)

	return chapter, nil
}

// SelectBoundsByVideoID выбирает границы всех глав видео, упорядоченные по start_ms, без
// привязки к пользователю (§3, §4 дизайна эпика Э4) — durationMs подставляется как конец
// последней главы. Видео без глав — пустой список, не ошибка.
func (r *ChapterRepository) SelectBoundsByVideoID(
	ctx context.Context, videoID uuid.UUID, durationMs int64,
) ([]domain.ChapterBound, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(chapterBoundsSQL, durationMs, videoID)

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[chapterBoundRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	bounds := make([]domain.ChapterBound, len(rows))
	for i, row := range rows {
		bounds[i] = row.toDomain()
	}

	return bounds, nil
}

// SelectProgressByVideoAndUser выбирает главы видео с покрытием одного пользователя (§3, §4
// дизайна эпика Э4) — durationMs подставляется как конец последней главы. Видео без глав —
// пустой список, не ошибка.
func (r *ChapterRepository) SelectProgressByVideoAndUser(
	ctx context.Context, videoID, userID uuid.UUID, durationMs int64,
) ([]domain.ChapterProgress, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(chapterProgressSQL, durationMs, videoID, userID, videoID)

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[chapterProgressRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	progress := make([]domain.ChapterProgress, len(rows))
	for i, row := range rows {
		progress[i] = row.toDomain()
	}

	return progress, nil
}

// chapterProgressRow — строка сканирования результата chapterProgressSQL (включает covered_ms
// в отличие от chapterBoundRow).
type chapterProgressRow struct {
	ChapterID uuid.UUID `db:"chapter_id"`
	VideoID   uuid.UUID `db:"video_id"`
	Name      string    `db:"name"`
	StartMs   int64     `db:"start_ms"`
	CreatedAt time.Time `db:"created_at"`
	EndMs     int64     `db:"end_ms"`
	CoveredMs int64     `db:"covered_ms"`
}

func (row chapterProgressRow) toDomain() domain.ChapterProgress {
	return domain.ChapterProgress{
		ChapterBound: domain.ChapterBound{
			Chapter: domain.Chapter{
				ID:        row.ChapterID,
				VideoID:   row.VideoID,
				Name:      row.Name,
				StartMs:   row.StartMs,
				CreatedAt: row.CreatedAt,
			},
			EndMs: row.EndMs,
		},
		CoveredMs: row.CoveredMs,
	}
}

// SelectProgressByVideoAndUsers батчем выбирает главы видео с покрытием сразу многих
// пользователей (§3, §4 дизайна эпика Э4, отчёты по назначению/сотруднику) — один SQL-запрос
// на всю карточку вместо запроса на участника (Н1). Пустой список идентификаторов не
// порождает запроса к БД. Пользователи без строки watch_progress по этому видео (ещё не
// смотрели) в результате не участвуют — вызывающая сторона трактует их как нулевое покрытие
// по всем главам без обращения к БД.
func (r *ChapterRepository) SelectProgressByVideoAndUsers(
	ctx context.Context, videoID uuid.UUID, userIDs []uuid.UUID, durationMs int64,
) ([]domain.ChapterUserProgress, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	// chapterUserProgressLeadingArgs — число позиционных аргументов CTE перед списком
	// идентификаторов пользователей (durationMs, videoID).
	const chapterUserProgressLeadingArgs = 2

	placeholders := make([]string, len(userIDs))
	args := make([]any, 0, len(userIDs)+chapterUserProgressLeadingArgs)
	args = append(args, durationMs, videoID)
	for i, userID := range userIDs {
		placeholders[i] = "?"
		args = append(args, userID)
	}

	sqlText := fmt.Sprintf(chapterUserProgressSQLTemplate, strings.Join(placeholders, ", "))

	rows, err := bob.All(ctx, exec, psql.RawQuery(sqlText, args...), scan.StructMapper[chapterUserProgressRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	progress := make([]domain.ChapterUserProgress, len(rows))
	for i, row := range rows {
		progress[i] = domain.ChapterUserProgress{
			ChapterProgress: domain.ChapterProgress{
				ChapterBound: domain.ChapterBound{
					Chapter: domain.Chapter{
						ID:        row.ChapterID,
						VideoID:   row.VideoID,
						Name:      row.Name,
						StartMs:   row.StartMs,
						CreatedAt: row.CreatedAt,
					},
					EndMs: row.EndMs,
				},
				CoveredMs: row.CoveredMs,
			},
			UserID: row.UserID,
		}
	}

	return progress, nil
}

// Update меняет начало и/или название главы. Конфликт по UNIQUE(video_id, start_ms) не
// перехватывается здесь — вызывающий сервис распознаёт его через dberrors.VideoChapterErrors и
// превращает в ErrChapterStartTaken (§4 дизайна эпика Э4). Строка не найдена — ErrNotFound.
func (r *ChapterRepository) Update(
	ctx context.Context, chapterID uuid.UUID, patch domain.ChapterPatch,
) (domain.Chapter, error) {
	exec := r.provider.GetExecutor(ctx)

	chapterDB, err := schema.VideoChapters.Query(
		sm.Where(schema.VideoChapters.Columns.ChapterID.EQ(psql.Arg(chapterID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Chapter{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Chapter{}, err
	}

	setter := &schema.VideoChapterSetter{}
	if patch.StartMs != nil {
		setter.StartMS = omit.From(*patch.StartMs)
	}
	if patch.Name != nil {
		setter.Name = omit.From(*patch.Name)
	}

	if err = chapterDB.Update(ctx, exec, setter); err != nil {
		zap.L().Error(err.Error())
		return domain.Chapter{}, err
	}

	var chapter domain.Chapter
	chapter.FromDB(chapterDB)

	return chapter, nil
}

// Delete удаляет главу видео. Соседние главы автоматически «наследуют» освободившийся
// промежуток при следующем чтении границ — дополнительного шага не требуется (§1 дизайна
// эпика Э4).
func (r *ChapterRepository) Delete(ctx context.Context, chapterID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.VideoChapters.Delete(
		dm.Where(schema.VideoChapters.Columns.ChapterID.EQ(psql.Arg(chapterID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}

// CountByVideoID считает число глав видео — используется для проверки лимита 100 глав на
// видео (Э4-Т3).
func (r *ChapterRepository) CountByVideoID(ctx context.Context, videoID uuid.UUID) (int, error) {
	exec := r.provider.GetExecutor(ctx)

	count, err := schema.VideoChapters.Query(
		sm.Where(schema.VideoChapters.Columns.VideoID.EQ(psql.Arg(videoID))),
	).Count(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return 0, err
	}

	return int(count), nil
}
