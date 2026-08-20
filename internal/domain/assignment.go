package domain

import (
	"encoding/json"
	"time"
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

// AssignmentDueMode определяет режим расчёта срока назначения (§1.1 дизайна эпика Э3).
type AssignmentDueMode string

const (
	// AssignmentDueModeDate — фиксированная дата: участники, зачисленные после неё, не
	// назначаются (В-5 решение владельца).
	AssignmentDueModeDate AssignmentDueMode = "date"
	// AssignmentDueModeDays — N дней с момента зачисления участника (для новичков — от даты
	// добавления в группу).
	AssignmentDueModeDays AssignmentDueMode = "days"
)

// AssignmentStatus определяет статус назначения целиком.
type AssignmentStatus string

const (
	AssignmentStatusActive    AssignmentStatus = "active"
	AssignmentStatusCancelled AssignmentStatus = "cancelled"
)

// AssignmentCancelReason определяет причину отмены назначения целиком (для журнала/отчёта).
type AssignmentCancelReason string

const (
	AssignmentCancelReasonManual       AssignmentCancelReason = "manual"
	AssignmentCancelReasonVideoDeleted AssignmentCancelReason = "video_deleted"
	AssignmentCancelReasonGroupDeleted AssignmentCancelReason = "group_deleted"
)

// AssignmentTargetType определяет тип цели назначения.
type AssignmentTargetType string

const (
	AssignmentTargetTypeUser  AssignmentTargetType = "user"
	AssignmentTargetTypeGroup AssignmentTargetType = "group"
)

// AssignmentParticipantStatus определяет статус персональной записи участника назначения.
type AssignmentParticipantStatus string

const (
	AssignmentParticipantStatusAssigned   AssignmentParticipantStatus = "assigned"
	AssignmentParticipantStatusInProgress AssignmentParticipantStatus = "in_progress"
	AssignmentParticipantStatusCompleted  AssignmentParticipantStatus = "completed"
	AssignmentParticipantStatusCancelled  AssignmentParticipantStatus = "cancelled"
)

// AssignmentParticipantSource определяет, как участник попал в назначение.
type AssignmentParticipantSource string

const (
	// AssignmentParticipantSourcePersonal — участник назначен лично (в списке Users).
	AssignmentParticipantSourcePersonal AssignmentParticipantSource = "personal"
	// AssignmentParticipantSourceGroup — участник зачислен через членство в группе-цели.
	AssignmentParticipantSourceGroup AssignmentParticipantSource = "group"
)

// AssignmentParticipantCancelReason определяет причину отмены персонального участия.
type AssignmentParticipantCancelReason string

const (
	AssignmentParticipantCancelReasonAssignmentCancelled AssignmentParticipantCancelReason = "assignment_cancelled"
	AssignmentParticipantCancelReasonRemovedByManager    AssignmentParticipantCancelReason = "removed_by_manager"
	AssignmentParticipantCancelReasonLeftGroup           AssignmentParticipantCancelReason = "left_group"
	AssignmentParticipantCancelReasonVideoDeleted        AssignmentParticipantCancelReason = "video_deleted"
	AssignmentParticipantCancelReasonGroupDeleted        AssignmentParticipantCancelReason = "group_deleted"
)

// AssignmentEventType определяет тип события журнала назначения (Э3-Т32).
type AssignmentEventType string

const (
	AssignmentEventTypeCreated              AssignmentEventType = "created"
	AssignmentEventTypeDueChanged           AssignmentEventType = "due_changed"
	AssignmentEventTypeCancelled            AssignmentEventType = "cancelled"
	AssignmentEventTypeParticipantEnrolled  AssignmentEventType = "participant_enrolled"
	AssignmentEventTypeParticipantCancelled AssignmentEventType = "participant_cancelled"
	AssignmentEventTypeParticipantCompleted AssignmentEventType = "participant_completed"
	AssignmentEventTypeParticipantRejected  AssignmentEventType = "participant_rejected"
)

// RejectedReason определяет причину, по которой цель не была включена в назначение (В-4).
type RejectedReason string

const (
	RejectedReasonInactive     RejectedReason = "inactive"
	RejectedReasonNoAccess     RejectedReason = "no_access"
	RejectedReasonNotInAccount RejectedReason = "not_in_account"
)

// Assignment — назначение обязательного обучения (видео) сотрудникам и/или группе (§1.1
// дизайна эпика Э3). VideoID/GroupID становятся nil при удалении видео/группы — снимки имени
// (VideoName/GroupName) остаются, работает История.
type Assignment struct {
	ID           uuid.UUID
	AccountID    uuid.UUID
	VideoID      *uuid.UUID
	VideoName    string
	GroupID      *uuid.UUID
	GroupName    string
	CreatedBy    uuid.UUID
	CreatedAt    time.Time
	DueMode      AssignmentDueMode
	DueAt        *time.Time
	DueDays      *int
	Comment      string
	Status       AssignmentStatus
	CancelledAt  *time.Time
	CancelledBy  *uuid.UUID
	CancelReason *AssignmentCancelReason
}

// FromDB заполняет Assignment строкой сгенерированной модели schema.Assignment.
func (a *Assignment) FromDB(db *schema.Assignment) {
	a.ID = db.AssignmentID
	a.AccountID = db.AccountID
	a.VideoID = dbconv.NullValToPtr(db.VideoID)
	a.VideoName = db.VideoName
	a.GroupID = dbconv.NullValToPtr(db.GroupID)
	a.GroupName = db.GroupName
	a.CreatedBy = db.CreatedBy
	a.CreatedAt = db.CreatedAt
	a.DueMode = AssignmentDueMode(db.DueMode)
	a.DueAt = dbconv.NullValToPtr(db.DueAt)
	a.Comment = db.Comment.GetOrZero()
	a.Status = AssignmentStatus(db.Status)
	a.CancelledAt = dbconv.NullValToPtr(db.CancelledAt)
	a.CancelledBy = dbconv.NullValToPtr(db.CancelledBy)

	if p := db.DueDays.Ptr(); p != nil {
		dueDays := int(*p)
		a.DueDays = &dueDays
	}
	if p := db.CancelReason.Ptr(); p != nil {
		reason := AssignmentCancelReason(*p)
		a.CancelReason = &reason
	}
}

// AssignmentTarget — цель назначения: конкретный пользователь или группа (§1.2 дизайна эпика Э3).
type AssignmentTarget struct {
	AssignmentID uuid.UUID
	TargetType   AssignmentTargetType
	TargetID     uuid.UUID
	// Name — отображаемое имя цели (ФИО пользователя или название группы), резолвится
	// сервисом при сборке карточки (AssignmentService.Get) — не хранится в БД, всегда пусто
	// сразу после FromDB.
	Name string
}

// FromDB заполняет AssignmentTarget строкой сгенерированной модели schema.AssignmentTarget.
func (t *AssignmentTarget) FromDB(db *schema.AssignmentTarget) {
	t.AssignmentID = db.AssignmentID
	t.TargetType = AssignmentTargetType(db.TargetType)
	t.TargetID = db.TargetID
}

// AssignmentParticipant — персональная запись участника назначения: прогресс и срок одного
// сотрудника (§1.3 дизайна эпика Э3, Э3-Н1 «неизменяемость подтверждения»).
type AssignmentParticipant struct {
	AssignmentID          uuid.UUID
	UserID                uuid.UUID
	Status                AssignmentParticipantStatus
	Source                AssignmentParticipantSource
	SourceGroupID         *uuid.UUID
	EnrolledAt            time.Time
	DueAt                 time.Time
	CompletedAt           *time.Time
	CompletedCoveragePct  *int
	CompletedThresholdPct *int
	CompletedSessionID    *uuid.UUID
	CancelledAt           *time.Time
	CancelReason          *AssignmentParticipantCancelReason
}

// FromDB заполняет AssignmentParticipant строкой сгенерированной модели
// schema.AssignmentParticipant.
func (p *AssignmentParticipant) FromDB(db *schema.AssignmentParticipant) {
	p.AssignmentID = db.AssignmentID
	p.UserID = db.UserID
	p.Status = AssignmentParticipantStatus(db.Status)
	p.Source = AssignmentParticipantSource(db.Source)
	p.SourceGroupID = dbconv.NullValToPtr(db.SourceGroupID)
	p.EnrolledAt = db.EnrolledAt
	p.DueAt = db.DueAt
	p.CompletedAt = dbconv.NullValToPtr(db.CompletedAt)
	p.CompletedSessionID = dbconv.NullValToPtr(db.CompletedSessionID)
	p.CancelledAt = dbconv.NullValToPtr(db.CancelledAt)

	if v := db.CompletedCoveragePCT.Ptr(); v != nil {
		pct := int(*v)
		p.CompletedCoveragePct = &pct
	}
	if v := db.CompletedThresholdPCT.Ptr(); v != nil {
		pct := int(*v)
		p.CompletedThresholdPct = &pct
	}
	if v := db.CancelReason.Ptr(); v != nil {
		reason := AssignmentParticipantCancelReason(*v)
		p.CancelReason = &reason
	}
}

// IsOverdue возвращает true, если участник ещё не завершил и не отменил обучение, а срок уже
// прошёл (Э3-Т4: просроченность — производное состояние, не хранится в БД).
func (p *AssignmentParticipant) IsOverdue(now time.Time) bool {
	active := p.Status == AssignmentParticipantStatusAssigned || p.Status == AssignmentParticipantStatusInProgress
	return active && p.DueAt.Before(now)
}

// CompletedLate возвращает true, если участник завершил обучение позже персонального срока.
func (p *AssignmentParticipant) CompletedLate() bool {
	return p.CompletedAt != nil && p.CompletedAt.After(p.DueAt)
}

// AssignmentEvent — запись журнала назначения (§1.6 дизайна эпика Э3, Э3-Т32).
type AssignmentEvent struct {
	ID           int64
	AssignmentID uuid.UUID
	UserID       *uuid.UUID
	Type         AssignmentEventType
	ActorID      *uuid.UUID
	Payload      json.RawMessage
	CreatedAt    time.Time
}

// FromDB заполняет AssignmentEvent строкой сгенерированной модели schema.AssignmentEvent.
func (e *AssignmentEvent) FromDB(db *schema.AssignmentEvent) {
	e.ID = db.EventID
	e.AssignmentID = db.AssignmentID
	e.UserID = dbconv.NullValToPtr(db.UserID)
	e.Type = AssignmentEventType(db.Type)
	e.ActorID = dbconv.NullValToPtr(db.ActorID)
	e.Payload = db.Payload.Val
	e.CreatedAt = db.CreatedAt
}

// AssignmentCounters — агрегированные счётчики участников назначения (§5 дизайна эпика Э3):
// собираются одним GROUP BY, без построчного подсчёта в Go.
type AssignmentCounters struct {
	Total      int
	Assigned   int
	InProgress int
	Completed  int
	Cancelled  int
	Overdue    int
}

// ParticipantDetails — участник назначения вместе с данными пользователя для карточки отчёта
// (§4 дизайна эпика Э3): CoveragePct — completed_coverage_pct для завершивших, иначе текущий
// прогресс просмотра; HasAccess — доступен ли участнику видео на текущий момент.
type ParticipantDetails struct {
	Participant AssignmentParticipant
	User        User
	CoveragePct int
	HasAccess   bool
}

// EventDetails — событие журнала назначения вместе с данными инициировавшего пользователя
// (§4 дизайна эпика Э3): Actor == nil — событие сгенерировано системой (heartbeat, каскад).
type EventDetails struct {
	Event AssignmentEvent
	Actor *User
}

// AssignmentDetails — карточка назначения целиком: само назначение, цели, счётчики, участники
// и журнал событий (§4 дизайна эпика Э3, ручка GET .../assignments/{id}).
type AssignmentDetails struct {
	Assignment Assignment
	// CreatedByUser — данные назначившего (§5 контракта эпика Э3), резолвится сервисом одним
	// запросом вместе с остальными пользователями карточки.
	CreatedByUser User
	Targets       []AssignmentTarget
	Counters      AssignmentCounters
	Participants  []ParticipantDetails
	Events        []EventDetails
}

// MyAssignment — назначение в разрезе одного участника для ручки GET me/assignments (§4
// дизайна эпика Э3): Video — текущее состояние видео, nil при удалении (работает снимок в
// Assignment.VideoName/GroupName).
type MyAssignment struct {
	Participant AssignmentParticipant
	Assignment  Assignment
	Video       *Video
	AssignedBy  User
	CoveragePct int
}

// RejectedTarget — пользователь, которого не удалось включить в назначение при создании (В-4),
// вместе с причиной отказа. Имя/фамилия/email резолвятся сервисом из того же батча
// пользователей, что и проверка доступа — без повторного запроса на уровне DTO.
type RejectedTarget struct {
	UserID  uuid.UUID
	Name    string
	Surname string
	Email   string
	Reason  RejectedReason
}

// UpdateAssignment — патч изменения назначения (§4 дизайна эпика Э3,
// AssignmentService.UpdateDue): nil-поле означает «не менять». Срок меняется целиком —
// режим вместе со своим значением.
type UpdateAssignment struct {
	DueMode *AssignmentDueMode
	DueAt   *time.Time
	DueDays *int
	Comment *string
}

// HasDue сообщает, что патч меняет срок назначения.
func (u UpdateAssignment) HasDue() bool {
	return u.DueMode != nil
}

// CreateAssignment — вход для создания назначения (§5 дизайна эпика Э3): DueAt/DueDays
// заполняется по DueMode; Users/Groups — хотя бы один непустой список.
type CreateAssignment struct {
	VideoID uuid.UUID
	Users   []uuid.UUID
	Groups  []uuid.UUID
	DueMode AssignmentDueMode
	DueAt   *time.Time
	DueDays *int
	Comment string
}
