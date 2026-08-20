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

// assignmentsFromDB конвертирует строки БД в доменные назначения.
func assignmentsFromDB(rows []*schema.Assignment) []domain.Assignment {
	assignments := make([]domain.Assignment, len(rows))
	for i, row := range rows {
		assignments[i].FromDB(row)
	}

	return assignments
}
