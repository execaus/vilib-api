package repository

import (
	"context"
	"encoding/json"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/types"
	"go.uber.org/zap"
)

// AssignmentEventRepository реализует репозиторий журнала назначения (§1.6, Э3-Т32 дизайна
// эпика Э3).
type AssignmentEventRepository struct {
	provider *ExecutorProvider
}

func NewAssignmentEventRepository(provider *ExecutorProvider) *AssignmentEventRepository {
	return &AssignmentEventRepository{provider: provider}
}

// Insert создаёт одну запись журнала. userID/actorID — nil для событий, относящихся к
// назначению целиком или инициированных системой (heartbeat, каскад).
func (r *AssignmentEventRepository) Insert(
	ctx context.Context,
	assignmentID uuid.UUID, userID *uuid.UUID,
	eventType domain.AssignmentEventType, actorID *uuid.UUID,
	payload json.RawMessage, now time.Time,
) (domain.AssignmentEvent, error) {
	exec := r.provider.GetExecutor(ctx)

	if payload == nil {
		payload = json.RawMessage("{}")
	}

	eventDB, err := schema.AssignmentEvents.Insert(&schema.AssignmentEventSetter{
		AssignmentID: omit.From(assignmentID),
		UserID:       omitnull.FromPtr(userID),
		Type:         omit.From(string(eventType)),
		ActorID:      omitnull.FromPtr(actorID),
		Payload:      omit.From(types.NewJSON(payload)),
		CreatedAt:    omit.From(now),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.AssignmentEvent{}, err
	}

	var event domain.AssignmentEvent
	event.FromDB(eventDB)

	return event, nil
}

// InsertBatch создаёт несколько записей журнала одну за другой в текущей транзакции (§4
// дизайна эпика Э3: события created/participant_enrolled/participant_completed/
// participant_rejected при создании назначения).
func (r *AssignmentEventRepository) InsertBatch(
	ctx context.Context, events []domain.AssignmentEvent,
) ([]domain.AssignmentEvent, error) {
	inserted := make([]domain.AssignmentEvent, len(events))
	for i, event := range events {
		now := event.CreatedAt
		if now.IsZero() {
			now = time.Now()
		}

		created, err := r.Insert(ctx, event.AssignmentID, event.UserID, event.Type, event.ActorID, event.Payload, now)
		if err != nil {
			return nil, err
		}

		inserted[i] = created
	}

	return inserted, nil
}

// SelectByAssignmentID выбирает журнал назначения в хронологическом порядке (§1.6 дизайна
// эпика Э3: индекс (assignment_id, event_id) задаёт порядок).
func (r *AssignmentEventRepository) SelectByAssignmentID(
	ctx context.Context, assignmentID uuid.UUID,
) ([]domain.AssignmentEvent, error) {
	exec := r.provider.GetExecutor(ctx)

	eventsDB, err := schema.AssignmentEvents.Query(
		sm.Where(schema.AssignmentEvents.Columns.AssignmentID.EQ(psql.Arg(assignmentID))),
		sm.OrderBy(schema.AssignmentEvents.Columns.EventID),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	events := make([]domain.AssignmentEvent, len(eventsDB))
	for i, e := range eventsDB {
		events[i].FromDB(e)
	}

	return events, nil
}
