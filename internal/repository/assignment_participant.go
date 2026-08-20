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
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/scan"
	"go.uber.org/zap"
)

// AssignmentParticipantRepository реализует репозиторий персональных записей участников
// назначений (§1.3, §4 дизайна эпика Э3): создание/реактивация, выборки для карточки,
// «моих назначений» и счётчиков, а также переходы статусов алгоритма зачёта heartbeat'а
// (§3 шаг 7 дизайна эпика Э3, добавлены в В-50).
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

// InsertBatch создаёт или реактивирует персональные записи участников назначения (§4 дизайна
// эпика Э3, семантика «OnMembersAdded»). При конфликте по (assignment_id, user_id) — первичный
// ключ, Э3-Т2 — обновление применяется только к отменённым записям (WHERE status='cancelled'):
// реактивирует их новыми source/enrolled_at/due_at и снимает cancelled_at/cancel_reason.
// Конфликт с активной или завершённой записью не обновляет строку — RETURNING не отдаёт её, и
// она пропускается в результате без ошибки (неизменяемость completed_*, Э3-Н1). При создании
// нового назначения (assignment_id всегда новый) конфликтов не возникает — реализация общая
// с будущим OnMembersAdded (В-52).
func (r *AssignmentParticipantRepository) InsertBatch(
	ctx context.Context, participants []domain.AssignmentParticipant,
) ([]domain.AssignmentParticipant, error) {
	exec := r.provider.GetExecutor(ctx)

	inserted := make([]domain.AssignmentParticipant, 0, len(participants))
	for _, p := range participants {
		setter := &schema.AssignmentParticipantSetter{
			AssignmentID:  omit.From(p.AssignmentID),
			UserID:        omit.From(p.UserID),
			Status:        omit.From(string(p.Status)),
			Source:        omit.From(string(p.Source)),
			SourceGroupID: omitnull.FromPtr(p.SourceGroupID),
			EnrolledAt:    omit.From(p.EnrolledAt),
			DueAt:         omit.From(p.DueAt),
		}

		row, err := schema.AssignmentParticipants.Insert(
			setter,
			im.OnConflict("assignment_id", "user_id").DoUpdate(
				im.SetExcluded(
					"status", "source", "source_group_id", "enrolled_at", "due_at",
					"cancelled_at", "cancel_reason",
				),
				im.Where(schema.AssignmentParticipants.Columns.Status.EQ(
					psql.Arg(string(domain.AssignmentParticipantStatusCancelled)),
				)),
			),
		).One(ctx, exec)
		if err != nil {
			if errors.Is(pgx.ErrNoRows, err) {
				// DO UPDATE ... WHERE не сработал (строка уже существует и не cancelled,
				// например completed/active) — участник не тронут, это ожидаемое поведение.
				continue
			}
			zap.L().Error(err.Error())
			return nil, err
		}

		participant := domain.AssignmentParticipant{}
		participant.FromDB(row)
		inserted = append(inserted, participant)
	}

	return inserted, nil
}

// SelectByAssignmentIDs батчем выбирает участников нескольких назначений (карточка/список,
// §4 дизайна эпика Э3). Пустой список идентификаторов не порождает запроса к БД.
func (r *AssignmentParticipantRepository) SelectByAssignmentIDs(
	ctx context.Context, assignmentIDs []uuid.UUID,
) ([]domain.AssignmentParticipant, error) {
	if len(assignmentIDs) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	idArgs := make([]bob.Expression, len(assignmentIDs))
	for i, id := range assignmentIDs {
		idArgs[i] = psql.Arg(id)
	}

	rows, err := schema.AssignmentParticipants.Query(
		sm.Where(schema.AssignmentParticipants.Columns.AssignmentID.In(idArgs...)),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	participants := make([]domain.AssignmentParticipant, len(rows))
	for i, row := range rows {
		participants[i].FromDB(row)
	}

	return participants, nil
}

// SelectByUserID выбирает все персональные записи пользователя во всех статусах (§4 дизайна
// эпика Э3, AssignmentService.ListMine) — деление на активные/выполненные/отменённые
// выполняется на клиенте.
func (r *AssignmentParticipantRepository) SelectByUserID(
	ctx context.Context, userID uuid.UUID,
) ([]domain.AssignmentParticipant, error) {
	exec := r.provider.GetExecutor(ctx)

	rows, err := schema.AssignmentParticipants.Query(
		sm.Where(schema.AssignmentParticipants.Columns.UserID.EQ(psql.Arg(userID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	participants := make([]domain.AssignmentParticipant, len(rows))
	for i, row := range rows {
		participants[i].FromDB(row)
	}

	return participants, nil
}

// countByAssignmentIDsSQLTemplate — агрегат статусов участников по назначениям одним GROUP BY
// (§4 дизайна эпика Э3): overdue считает только незавершённых с истёкшим персональным сроком.
const countByAssignmentIDsSQLTemplate = `
SELECT assignment_id,
	count(*) FILTER (WHERE status = 'assigned')::int AS assigned,
	count(*) FILTER (WHERE status = 'in_progress')::int AS in_progress,
	count(*) FILTER (WHERE status = 'completed')::int AS completed,
	count(*) FILTER (WHERE status = 'cancelled')::int AS cancelled,
	count(*)::int AS total,
	count(*) FILTER (
		WHERE status IN ('assigned', 'in_progress') AND due_at < now()
	)::int AS overdue
FROM assignment_participants
WHERE assignment_id IN (%s)
GROUP BY assignment_id
`

// assignmentCounterRow — строка сканирования результата countByAssignmentIDsSQLTemplate.
type assignmentCounterRow struct {
	AssignmentID uuid.UUID `db:"assignment_id"`
	Assigned     int       `db:"assigned"`
	InProgress   int       `db:"in_progress"`
	Completed    int       `db:"completed"`
	Cancelled    int       `db:"cancelled"`
	Total        int       `db:"total"`
	Overdue      int       `db:"overdue"`
}

// CountByAssignmentIDs агрегирует счётчики статусов участников по каждому назначению одним
// запросом (§4 дизайна эпика Э3) — назначения без ни одной строки в результате не
// присутствуют в возвращённой карте (вызывающая сторона трактует отсутствие как нулевые
// счётчики). Пустой список идентификаторов не порождает запроса к БД.
func (r *AssignmentParticipantRepository) CountByAssignmentIDs(
	ctx context.Context, assignmentIDs []uuid.UUID,
) (map[uuid.UUID]domain.AssignmentCounters, error) {
	result := make(map[uuid.UUID]domain.AssignmentCounters, len(assignmentIDs))
	if len(assignmentIDs) == 0 {
		return result, nil
	}

	exec := r.provider.GetExecutor(ctx)

	placeholders := make([]string, len(assignmentIDs))
	args := make([]any, len(assignmentIDs))
	for i, id := range assignmentIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := psql.RawQuery(fmt.Sprintf(countByAssignmentIDsSQLTemplate, strings.Join(placeholders, ", ")), args...)

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[assignmentCounterRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	for _, row := range rows {
		result[row.AssignmentID] = domain.AssignmentCounters{
			Total:      row.Total,
			Assigned:   row.Assigned,
			InProgress: row.InProgress,
			Completed:  row.Completed,
			Cancelled:  row.Cancelled,
			Overdue:    row.Overdue,
		}
	}

	return result, nil
}
