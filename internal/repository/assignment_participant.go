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
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
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
// JOIN на users нужен только для фильтра включения деактивированных (В-53) — единственный JOIN
// в отчёте, оправдан дизайном (§4).
const countByAssignmentIDsSQLTemplate = `
SELECT ap.assignment_id,
	count(*) FILTER (WHERE ap.status = 'assigned')::int AS assigned,
	count(*) FILTER (WHERE ap.status = 'in_progress')::int AS in_progress,
	count(*) FILTER (WHERE ap.status = 'completed')::int AS completed,
	count(*) FILTER (WHERE ap.status = 'cancelled')::int AS cancelled,
	count(*)::int AS total,
	count(*) FILTER (
		WHERE ap.status IN ('assigned', 'in_progress') AND ap.due_at < now()
	)::int AS overdue
FROM assignment_participants ap
INNER JOIN users u ON u.user_id = ap.user_id
WHERE ap.assignment_id IN (%s)
  AND (? OR u.deactivated_at IS NULL)
GROUP BY ap.assignment_id
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
// счётчики). includeDeactivated=false исключает из счётчиков участников с деактивированной
// строкой пользователя (В-53, критерий Э3-Т29/КП-8). Пустой список идентификаторов не
// порождает запроса к БД.
func (r *AssignmentParticipantRepository) CountByAssignmentIDs(
	ctx context.Context, assignmentIDs []uuid.UUID, includeDeactivated bool,
) (map[uuid.UUID]domain.AssignmentCounters, error) {
	result := make(map[uuid.UUID]domain.AssignmentCounters, len(assignmentIDs))
	if len(assignmentIDs) == 0 {
		return result, nil
	}

	exec := r.provider.GetExecutor(ctx)

	placeholders := make([]string, len(assignmentIDs))
	args := make([]any, len(assignmentIDs), len(assignmentIDs)+1)
	for i, id := range assignmentIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	args = append(args, includeDeactivated)

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

// userIDRow — строка сканирования результата RETURNING user_id.
type userIDRow struct {
	UserID uuid.UUID `db:"user_id"`
}

func userIDsFromRows(rows []userIDRow) []uuid.UUID {
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.UserID
	}

	return ids
}

// SelectByAssignmentIDAndUserID выбирает персональную запись участника назначения (§4 дизайна
// эпика Э3, AssignmentService.RemoveParticipant). Строка не найдена — ErrNotFound.
func (r *AssignmentParticipantRepository) SelectByAssignmentIDAndUserID(
	ctx context.Context, assignmentID, userID uuid.UUID,
) (domain.AssignmentParticipant, error) {
	exec := r.provider.GetExecutor(ctx)

	row, err := schema.AssignmentParticipants.Query(
		sm.Where(schema.AssignmentParticipants.Columns.AssignmentID.EQ(psql.Arg(assignmentID))),
		sm.Where(schema.AssignmentParticipants.Columns.UserID.EQ(psql.Arg(userID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.AssignmentParticipant{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.AssignmentParticipant{}, err
	}

	var participant domain.AssignmentParticipant
	participant.FromDB(row)

	return participant, nil
}

const updateDueByAssignmentDateSQL = `
UPDATE assignment_participants
SET due_at = ?
WHERE assignment_id = ?
  AND status IN ('assigned', 'in_progress')
RETURNING user_id
`

const updateDueByAssignmentDaysSQL = `
UPDATE assignment_participants
SET due_at = enrolled_at + (? * interval '1 day')
WHERE assignment_id = ?
  AND status IN ('assigned', 'in_progress')
RETURNING user_id
`

// UpdateDueByAssignment пересчитывает персональные сроки незавершённых участников назначения
// (§4 дизайна эпика Э3, AssignmentService.UpdateDue): режим date ставит всем общий срок,
// режим days — enrolled_at + dueDays. Условие по статусу в самом запросе оставляет
// завершённые (Э3-Н1) и отменённые записи нетронутыми.
func (r *AssignmentParticipantRepository) UpdateDueByAssignment(
	ctx context.Context,
	assignmentID uuid.UUID,
	dueMode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	var query bob.BaseQuery[expr.Clause]
	switch {
	case dueMode == domain.AssignmentDueModeDate && dueAt != nil:
		query = psql.RawQuery(updateDueByAssignmentDateSQL, *dueAt, assignmentID)
	case dueMode == domain.AssignmentDueModeDays && dueDays != nil:
		query = psql.RawQuery(updateDueByAssignmentDaysSQL, *dueDays, assignmentID)
	default:
		return nil, fmt.Errorf("%w: режим срока %q без значения", ErrInvalidDueMode, dueMode)
	}

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[userIDRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return userIDsFromRows(rows), nil
}

const cancelByAssignmentSQL = `
UPDATE assignment_participants
SET status = 'cancelled',
	cancelled_at = ?,
	cancel_reason = ?
WHERE assignment_id = ?
  AND status IN ('assigned', 'in_progress')
RETURNING user_id
`

// CancelByAssignment отменяет незавершённых участников назначения (§4 дизайна эпика Э3:
// отмена назначения, удаление видео или группы) и возвращает id отменённых пользователей.
// Завершённые записи остаются как есть (Э3-Н1).
func (r *AssignmentParticipantRepository) CancelByAssignment(
	ctx context.Context,
	assignmentID uuid.UUID,
	reason domain.AssignmentParticipantCancelReason, at time.Time,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(cancelByAssignmentSQL, at, string(reason), assignmentID)

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[userIDRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return userIDsFromRows(rows), nil
}

// CancelOne отменяет участие одного пользователя в назначении (§4 дизайна эпика Э3, снятие
// участника менеджером). Условие по статусу в запросе оставляет завершённую запись нетронутой
// — в этом случае метод возвращает false.
func (r *AssignmentParticipantRepository) CancelOne(
	ctx context.Context,
	assignmentID, userID uuid.UUID,
	reason domain.AssignmentParticipantCancelReason, at time.Time,
) (bool, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.AssignmentParticipantSetter{
		Status:       omit.From(string(domain.AssignmentParticipantStatusCancelled)),
		CancelledAt:  omitnull.From(at),
		CancelReason: omitnull.From(string(reason)),
	}

	affected, err := schema.AssignmentParticipants.Update(
		setter.UpdateMod(),
		um.Where(schema.AssignmentParticipants.Columns.AssignmentID.EQ(psql.Arg(assignmentID))),
		um.Where(schema.AssignmentParticipants.Columns.UserID.EQ(psql.Arg(userID))),
		um.Where(schema.AssignmentParticipants.Columns.Status.In(
			psql.Arg(string(domain.AssignmentParticipantStatusAssigned)),
			psql.Arg(string(domain.AssignmentParticipantStatusInProgress)),
		)),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return false, err
	}

	return affected > 0, nil
}

const cancelBySourceGroupAndUserSQL = `
UPDATE assignment_participants ap
SET status = 'cancelled',
	cancelled_at = ?,
	cancel_reason = ?
FROM assignments a
WHERE ap.assignment_id = a.assignment_id
  AND a.status = 'active'
  AND ap.user_id = ?
  AND ap.source = 'group'
  AND ap.source_group_id = ?
  AND ap.status IN ('assigned', 'in_progress')
RETURNING ap.assignment_id
`

// CancelBySourceGroupAndUser отменяет участия пользователя, полученные через членство в
// группе, во всех действующих назначениях (§4 дизайна эпика Э3, каскад OnMemberRemoved).
// Личные назначения (source=personal) не затрагиваются — Э3-Т30. Возвращает id назначений,
// в которых участие отменено.
func (r *AssignmentParticipantRepository) CancelBySourceGroupAndUser(
	ctx context.Context,
	groupID, userID uuid.UUID,
	reason domain.AssignmentParticipantCancelReason, at time.Time,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	query := psql.RawQuery(cancelBySourceGroupAndUserSQL, at, string(reason), userID, groupID)

	rows, err := bob.All(ctx, exec, query, scan.StructMapper[assignmentIDRow]())
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentIDsFromRows(rows), nil
}
