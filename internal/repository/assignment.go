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
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

// AssignmentRepository реализует репозиторий назначений обязательного обучения (§1.1, §4
// дизайна эпика Э3).
type AssignmentRepository struct {
	provider *ExecutorProvider
}

func NewAssignmentRepository(provider *ExecutorProvider) *AssignmentRepository {
	return &AssignmentRepository{provider: provider}
}

// Insert создаёт назначение в статусе active (§4 шаг 6 дизайна эпика Э3). videoName/groupName
// — снимок названия видео и группы на момент создания (Э3-Т7).
func (r *AssignmentRepository) Insert(
	ctx context.Context,
	accountID, videoID uuid.UUID, videoName string,
	groupID uuid.UUID, groupName string,
	createdBy uuid.UUID,
	dueMode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int,
	comment string,
) (domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.AssignmentSetter{
		AccountID: omit.From(accountID),
		VideoID:   omitnull.From(videoID),
		VideoName: omit.From(videoName),
		GroupID:   omitnull.From(groupID),
		GroupName: omit.From(groupName),
		CreatedBy: omit.From(createdBy),
		CreatedAt: omit.From(time.Now()),
		DueMode:   omit.From(string(dueMode)),
		Status:    omit.From(string(domain.AssignmentStatusActive)),
	}
	if dueAt != nil {
		setter.DueAt = omitnull.From(*dueAt)
	}
	if dueDays != nil {
		setter.DueDays = omitnull.From(int32FromInt(*dueDays))
	}
	if comment != "" {
		setter.Comment = omitnull.From(comment)
	}

	assignmentDB, err := schema.Assignments.Insert(setter).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Assignment{}, err
	}

	var assignment domain.Assignment
	assignment.FromDB(assignmentDB)

	return assignment, nil
}

// SelectByID выбирает назначение по идентификатору. Строка не найдена — ErrNotFound.
func (r *AssignmentRepository) SelectByID(ctx context.Context, id uuid.UUID) (domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	assignmentDB, err := schema.Assignments.Query(
		sm.Where(schema.Assignments.Columns.AssignmentID.EQ(psql.Arg(id))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Assignment{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Assignment{}, err
	}

	var assignment domain.Assignment
	assignment.FromDB(assignmentDB)

	return assignment, nil
}

// SelectByIDs батчем выбирает назначения по списку идентификаторов (§4 дизайна эпика Э3,
// AssignmentService.ListMine). Отсутствие строки для части id — не ошибка. Пустой список
// идентификаторов не порождает запроса к БД.
func (r *AssignmentRepository) SelectByIDs(ctx context.Context, ids []uuid.UUID) ([]domain.Assignment, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	idArgs := make([]bob.Expression, len(ids))
	for i, id := range ids {
		idArgs[i] = psql.Arg(id)
	}

	assignmentsDB, err := schema.Assignments.Query(
		sm.Where(schema.Assignments.Columns.AssignmentID.In(idArgs...)),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	assignments := make([]domain.Assignment, len(assignmentsDB))
	for i, a := range assignmentsDB {
		assignments[i].FromDB(a)
	}

	return assignments, nil
}

// UpdateDue меняет режим и значение срока назначения (§4 дизайна эпика Э3,
// AssignmentService.UpdateDue): заполняется поле выбранного режима, противоположное
// обнуляется — ограничение chk_assignments_due допускает только согласованную пару.
func (r *AssignmentRepository) UpdateDue(
	ctx context.Context,
	id uuid.UUID,
	dueMode domain.AssignmentDueMode, dueAt *time.Time, dueDays *int,
) (domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.AssignmentSetter{
		DueMode: omit.From(string(dueMode)),
		DueAt:   omitnull.FromPtr(dueAt),
		DueDays: omitnull.FromPtr[int32](nil),
	}
	if dueDays != nil {
		setter.DueDays = omitnull.From(int32FromInt(*dueDays))
	}

	assignmentDB, err := schema.Assignments.Update(
		setter.UpdateMod(),
		um.Where(schema.Assignments.Columns.AssignmentID.EQ(psql.Arg(id))),
		um.Returning("*"),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Assignment{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Assignment{}, err
	}

	var assignment domain.Assignment
	assignment.FromDB(assignmentDB)

	return assignment, nil
}

// UpdateComment меняет комментарий назначения; пустая строка очищает его (колонка nullable).
func (r *AssignmentRepository) UpdateComment(
	ctx context.Context, id uuid.UUID, comment string,
) (domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.AssignmentSetter{Comment: omitnull.FromPtr[string](nil)}
	if comment != "" {
		setter.Comment = omitnull.From(comment)
	}

	assignmentDB, err := schema.Assignments.Update(
		setter.UpdateMod(),
		um.Where(schema.Assignments.Columns.AssignmentID.EQ(psql.Arg(id))),
		um.Returning("*"),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Assignment{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Assignment{}, err
	}

	var assignment domain.Assignment
	assignment.FromDB(assignmentDB)

	return assignment, nil
}

// Cancel переводит назначение в статус cancelled (§4 дизайна эпика Э3). Условие
// status='active' в самом запросе защищает от повторной отмены: уже отменённое назначение не
// обновляется и метод возвращает false. cancelledBy — nil для системных отмен (удаление видео
// или группы).
func (r *AssignmentRepository) Cancel(
	ctx context.Context,
	id uuid.UUID, cancelledBy *uuid.UUID,
	reason domain.AssignmentCancelReason, at time.Time,
) (bool, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.AssignmentSetter{
		Status:       omit.From(string(domain.AssignmentStatusCancelled)),
		CancelledAt:  omitnull.From(at),
		CancelledBy:  omitnull.FromPtr(cancelledBy),
		CancelReason: omitnull.From(string(reason)),
	}

	affected, err := schema.Assignments.Update(
		setter.UpdateMod(),
		um.Where(schema.Assignments.Columns.AssignmentID.EQ(psql.Arg(id))),
		um.Where(schema.Assignments.Columns.Status.EQ(psql.Arg(string(domain.AssignmentStatusActive)))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return false, err
	}

	return affected > 0, nil
}

// SelectActiveByTargetGroup выбирает действующие назначения, адресованные группе как цели
// (§4 дизайна эпика Э3, каскад OnMembersAdded).
func (r *AssignmentRepository) SelectActiveByTargetGroup(
	ctx context.Context, groupID uuid.UUID,
) ([]domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	assignmentsDB, err := schema.Assignments.Query(
		sm.InnerJoin(schema.AssignmentTargets.Name()).OnEQ(
			schema.AssignmentTargets.Columns.AssignmentID,
			schema.Assignments.Columns.AssignmentID,
		),
		sm.Where(schema.AssignmentTargets.Columns.TargetType.EQ(
			psql.Arg(string(domain.AssignmentTargetTypeGroup)),
		)),
		sm.Where(schema.AssignmentTargets.Columns.TargetID.EQ(psql.Arg(groupID))),
		sm.Where(schema.Assignments.Columns.Status.EQ(psql.Arg(string(domain.AssignmentStatusActive)))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentsFromDB(assignmentsDB), nil
}

// SelectActiveByVideoIDs выбирает действующие назначения перечисленных видео (§4 дизайна
// эпика Э3, каскад OnVideoDeleted). Пустой список идентификаторов не порождает запроса к БД.
func (r *AssignmentRepository) SelectActiveByVideoIDs(
	ctx context.Context, videoIDs []uuid.UUID,
) ([]domain.Assignment, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	idArgs := make([]bob.Expression, len(videoIDs))
	for i, id := range videoIDs {
		idArgs[i] = psql.Arg(id)
	}

	assignmentsDB, err := schema.Assignments.Query(
		sm.Where(schema.Assignments.Columns.VideoID.In(idArgs...)),
		sm.Where(schema.Assignments.Columns.Status.EQ(psql.Arg(string(domain.AssignmentStatusActive)))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentsFromDB(assignmentsDB), nil
}

// SelectActiveByGroupID выбирает действующие назначения видео указанной группы (§4 дизайна
// эпика Э3, каскад OnGroupDeleted).
func (r *AssignmentRepository) SelectActiveByGroupID(
	ctx context.Context, groupID uuid.UUID,
) ([]domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	assignmentsDB, err := schema.Assignments.Query(
		sm.Where(schema.Assignments.Columns.GroupID.EQ(psql.Arg(groupID))),
		sm.Where(schema.Assignments.Columns.Status.EQ(psql.Arg(string(domain.AssignmentStatusActive)))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentsFromDB(assignmentsDB), nil
}

// assignmentScopeExpr переводит область В-8 (AssignmentScope) в SQL-условие: назначение видно,
// если область — весь аккаунт, либо оно принадлежит одной из групп области, либо его создал
// сам инициатор (собственные назначения видны независимо от области, §2 дизайна эпика Э3).
func assignmentScopeExpr(scope AssignmentScope) psql.Expression {
	createdByExpr := schema.Assignments.Columns.CreatedBy.EQ(psql.Arg(scope.CreatedBy))
	if scope.All {
		return createdByExpr
	}
	if len(scope.GroupIDs) == 0 {
		return createdByExpr
	}

	groupArgs := make([]bob.Expression, len(scope.GroupIDs))
	for i, id := range scope.GroupIDs {
		groupArgs[i] = psql.Arg(id)
	}

	return psql.Or(schema.Assignments.Columns.GroupID.In(groupArgs...), createdByExpr)
}

// assignmentDueRangeExpr строит условие фильтра периода (due_from/due_to, В-53/В-61):
// назначение попадает в период, если срок в границах [dueFrom; dueTo] (включительно) есть
// либо у самого назначения (assignments.due_at, режим «дата»), либо у персонального срока
// хотя бы одного незавершённого участника (assignment_participants.due_at, режим «N дней с
// зачисления» — там срок персональный и в assignments.due_at не хранится). Отменённые
// участия (status='cancelled') в проверку не берутся: отменённое участие не создаёт
// обязанности пройти видео в периоде. Границы независимы — отсутствующая не ограничивает
// соответствующую сторону диапазона; ровно так же, как раньше вела себя проверка по
// assignments.due_at.
func assignmentDueRangeExpr(dueFrom, dueTo *time.Time) psql.Expression {
	var assignmentConds, participantConds []string
	var args []any

	if dueFrom != nil {
		assignmentConds = append(assignmentConds, "assignments.due_at >= ?")
		participantConds = append(participantConds, "ap.due_at >= ?")
		args = append(args, *dueFrom)
	}
	if dueTo != nil {
		assignmentConds = append(assignmentConds, "assignments.due_at <= ?")
		participantConds = append(participantConds, "ap.due_at <= ?")
		args = append(args, *dueTo)
	}

	// args собираются в порядке появления плейсхолдеров: сначала условие по assignments.due_at,
	// затем — по ap.due_at внутри EXISTS, в обоих местах due_from раньше due_to.
	fullArgs := make([]any, 0, len(args)*2) //nolint:mnd // удвоение: условие встречается в обеих ветках OR
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, args...)

	// Внешние скобки обязательны: bob соединяет условия WHERE через AND без группировки,
	// и без них дизъюнкция разорвала бы соседние условия (в том числе account_id) по
	// приоритету операторов — назначения чужого аккаунта попали бы в выборку.
	sqlExpr := fmt.Sprintf(
		"((%s) OR EXISTS ("+
			"SELECT 1 FROM assignment_participants ap "+
			"WHERE ap.assignment_id = assignments.assignment_id "+
			"AND ap.status <> 'cancelled' AND (%s)))",
		strings.Join(assignmentConds, " AND "),
		strings.Join(participantConds, " AND "),
	)

	return psql.Raw(sqlExpr, fullArgs...)
}

// SelectByFilter выбирает назначения аккаунта по области В-8 и дополнительным фильтрам списка/
// отчёта (§4, §5 дизайна эпика Э3, AssignmentService.List/ListForUser). Область all=true не
// сужает выборку никаким условием сверх account_id — назначение видно целиком по аккаунтному
// праву. Фильтр периода due_from/due_to трактует срок по-разному в зависимости от режима
// назначения (assignmentDueRangeExpr): «дата» — assignments.due_at, «N дней с зачисления» —
// персональный срок участника.
func (r *AssignmentRepository) SelectByFilter(ctx context.Context, f AssignmentFilter) ([]domain.Assignment, error) {
	exec := r.provider.GetExecutor(ctx)

	mods := []bob.Mod[*dialect.SelectQuery]{
		sm.Where(schema.Assignments.Columns.AccountID.EQ(psql.Arg(f.AccountID))),
	}
	if !f.Scope.All {
		mods = append(mods, sm.Where(assignmentScopeExpr(f.Scope)))
	}
	if f.GroupID != nil {
		mods = append(mods, sm.Where(schema.Assignments.Columns.GroupID.EQ(psql.Arg(*f.GroupID))))
	}
	if f.VideoID != nil {
		mods = append(mods, sm.Where(schema.Assignments.Columns.VideoID.EQ(psql.Arg(*f.VideoID))))
	}
	if f.Status != nil {
		mods = append(mods, sm.Where(schema.Assignments.Columns.Status.EQ(psql.Arg(string(*f.Status)))))
	}
	if f.DueFrom != nil || f.DueTo != nil {
		mods = append(mods, sm.Where(assignmentDueRangeExpr(f.DueFrom, f.DueTo)))
	}
	if f.UserID != nil {
		mods = append(mods, sm.Where(psql.Raw(
			"exists (select 1 from assignment_participants ap "+
				"where ap.assignment_id = assignments.assignment_id and ap.user_id = ?)",
			*f.UserID,
		)))
	}

	assignmentsDB, err := schema.Assignments.Query(mods...).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return assignmentsFromDB(assignmentsDB), nil
}

// assignmentsFromDB конвертирует строки БД в доменные назначения.
func assignmentsFromDB(rows []*schema.Assignment) []domain.Assignment {
	assignments := make([]domain.Assignment, len(rows))
	for i, row := range rows {
		assignments[i].FromDB(row)
	}

	return assignments
}
