package repository

import (
	"context"
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/scan"
	"go.uber.org/zap"
)

// AssignmentParticipantRepository реализует минимальный срез репозитория персональных
// записей участников назначений, нужный алгоритму зачёта heartbeat'а (§3 шаг 7 дизайна эпика
// Э3). Остальные методы (Insert/Select/Cancel и т.д., §4 дизайна эпика) добавляются в В-51.
type AssignmentParticipantRepository struct {
	provider *ExecutorProvider
}

func NewAssignmentParticipantRepository(provider *ExecutorProvider) *AssignmentParticipantRepository {
	return &AssignmentParticipantRepository{provider: provider}
}

// assignmentIDRow — строка сканирования результата RETURNING assignment_id.
type assignmentIDRow struct {
	AssignmentID uuid.UUID `db:"assignment_id"`
}

func assignmentIDsFromRows(rows []assignmentIDRow) []uuid.UUID {
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.AssignmentID
	}

	return ids
}

// int16FromPct конвертирует процент (0..100) в int16 для хранения в БД — значение всегда
// контролируется сервисом и заведомо мало.
func int16FromPct(pct int) int16 {
	return int16(pct) // #nosec G115 -- процент покрытия/порога всегда в диапазоне 0..100
}

// nullableUUIDArg подготавливает *uuid.UUID к передаче как аргумент сырого SQL-запроса:
// nil-указатель нельзя передавать драйверу напрямую (uuid.UUID реализует driver.Valuer со
// значимым получателем, вызов метода на nil-указателе паникует), поэтому nil превращается в
// нетипизированный nil-интерфейс (SQL NULL), а непустой указатель — в значение uuid.UUID.
func nullableUUIDArg(id *uuid.UUID) any {
	if id == nil {
		return nil
	}

	return *id
}

const updateStatusByUserVideoSQL = `
UPDATE assignment_participants ap
SET status = ?
FROM assignments a
WHERE ap.assignment_id = a.assignment_id
  AND a.status = 'active'
  AND a.video_id = ?
  AND ap.user_id = ?
  AND ap.status = ?
RETURNING ap.assignment_id
`

// UpdateStatusByUserVideo переводит статус участника из from в to для всех активных
// назначений видео videoID, в которых участвует userID.
func (r *AssignmentParticipantRepository) UpdateStatusByUserVideo(
	ctx context.Context,
	userID, videoID uuid.UUID,
	from, to domain.AssignmentParticipantStatus,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(updateStatusByUserVideoSQL, string(to), videoID, userID, string(from))

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[assignmentIDRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentIDsFromRows(rows), nil
}

const completeByUserVideoSQL = `
UPDATE assignment_participants ap
SET status = 'completed',
	completed_at = ?,
	completed_coverage_pct = ?,
	completed_threshold_pct = ?,
	completed_session_id = ?
FROM assignments a
WHERE ap.assignment_id = a.assignment_id
  AND a.status = 'active'
  AND a.video_id = ?
  AND ap.user_id = ?
  AND ap.status IN ('assigned', 'in_progress')
RETURNING ap.assignment_id
`

// CompleteByUserVideo завершает участие userID во всех активных назначениях видео videoID,
// ещё не завершённых (assigned/in_progress) — условие WHERE в самом запросе гарантирует
// неизменяемость completed_* (Э3-Н1).
func (r *AssignmentParticipantRepository) CompleteByUserVideo(
	ctx context.Context,
	userID, videoID uuid.UUID,
	completedAt time.Time, coveragePct, thresholdPct int,
	sessionID *uuid.UUID,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(
		completeByUserVideoSQL,
		completedAt, int16FromPct(coveragePct), int16FromPct(thresholdPct), nullableUUIDArg(sessionID),
		videoID, userID,
	)

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[assignmentIDRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentIDsFromRows(rows), nil
}
