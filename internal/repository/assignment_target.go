package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

// AssignmentTargetRepository реализует репозиторий целей назначения — кому адресовано
// назначение (§1.2 дизайна эпика Э3): конкретный пользователь или группа.
type AssignmentTargetRepository struct {
	provider *ExecutorProvider
}

func NewAssignmentTargetRepository(provider *ExecutorProvider) *AssignmentTargetRepository {
	return &AssignmentTargetRepository{provider: provider}
}

// InsertBatch создаёt строки целей назначения. Уникальность (assignment_id, target_type,
// target_id) — первичный ключ, конфликтов при создании назначения не бывает (assignment_id
// всегда новый).
func (r *AssignmentTargetRepository) InsertBatch(
	ctx context.Context, targets []domain.AssignmentTarget,
) ([]domain.AssignmentTarget, error) {
	exec := r.provider.GetExecutor(ctx)

	inserted := make([]domain.AssignmentTarget, len(targets))
	for i, target := range targets {
		targetDB, err := schema.AssignmentTargets.Insert(&schema.AssignmentTargetSetter{
			AssignmentID: omit.From(target.AssignmentID),
			TargetType:   omit.From(string(target.TargetType)),
			TargetID:     omit.From(target.TargetID),
		}).One(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return nil, err
		}

		inserted[i] = domain.AssignmentTarget{}
		inserted[i].FromDB(targetDB)
	}

	return inserted, nil
}

// SelectByAssignmentIDs батчем выбирает цели нескольких назначений (карточка/список, §4
// дизайна эпика Э3). Пустой список идентификаторов не порождает запроса к БД.
func (r *AssignmentTargetRepository) SelectByAssignmentIDs(
	ctx context.Context, assignmentIDs []uuid.UUID,
) ([]domain.AssignmentTarget, error) {
	if len(assignmentIDs) == 0 {
		return nil, nil
	}

	exec := r.provider.GetExecutor(ctx)

	idArgs := make([]bob.Expression, len(assignmentIDs))
	for i, id := range assignmentIDs {
		idArgs[i] = psql.Arg(id)
	}

	targetsDB, err := schema.AssignmentTargets.Query(
		sm.Where(schema.AssignmentTargets.Columns.AssignmentID.In(idArgs...)),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	targets := make([]domain.AssignmentTarget, len(targetsDB))
	for i, t := range targetsDB {
		targets[i].FromDB(t)
	}

	return targets, nil
}
