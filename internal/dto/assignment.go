package dto

import (
	"encoding/json"
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

// AssignmentUser — краткие сведения о пользователе в контексте назначения: назначивший,
// участник, инициатор события журнала (§5 контракта эпика Э3).
type AssignmentUser struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Surname  string    `json:"surname"`
	Email    string    `json:"email"`
	IsActive bool      `json:"is_active"`
}

// FromDomain заполняет AssignmentUser доменным пользователем.
func (u *AssignmentUser) FromDomain(user domain.User) {
	u.ID = user.ID
	u.Name = user.Name
	u.Surname = user.Surname
	u.Email = user.Email
	u.IsActive = user.IsActive()
}

// AssignmentTarget — цель назначения: конкретный пользователь или группа (§5 контракта эпика
// Э3).
type AssignmentTarget struct {
	Type string    `json:"type"`
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// FromDomain заполняет AssignmentTarget доменной целью (Name резолвится сервисом,
// AssignmentService.Get).
func (t *AssignmentTarget) FromDomain(target domain.AssignmentTarget) {
	t.Type = string(target.TargetType)
	t.ID = target.TargetID
	t.Name = target.Name
}

// AssignmentCounters — агрегированные счётчики участников назначения (§5 контракта эпика Э3).
type AssignmentCounters struct {
	Total      int `json:"total"`
	Assigned   int `json:"assigned"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Cancelled  int `json:"cancelled"`
	Overdue    int `json:"overdue"`
}

// FromDomain заполняет AssignmentCounters доменными счётчиками.
func (c *AssignmentCounters) FromDomain(counters domain.AssignmentCounters) {
	c.Total = counters.Total
	c.Assigned = counters.Assigned
	c.InProgress = counters.InProgress
	c.Completed = counters.Completed
	c.Cancelled = counters.Cancelled
	c.Overdue = counters.Overdue
}

// Assignment — назначение обучения без персональных записей (§5 контракта эпика Э3).
type Assignment struct {
	ID           uuid.UUID          `json:"id"`
	VideoID      *uuid.UUID         `json:"video_id"`
	VideoName    string             `json:"video_name"`
	GroupID      *uuid.UUID         `json:"group_id"`
	GroupName    string             `json:"group_name"`
	CreatedBy    AssignmentUser     `json:"created_by"`
	CreatedAt    time.Time          `json:"created_at"`
	DueMode      string             `json:"due_mode"`
	DueAt        *time.Time         `json:"due_at,omitempty"`
	DueDays      *int               `json:"due_days,omitempty"`
	Comment      string             `json:"comment"`
	Status       string             `json:"status"`
	CancelledAt  *time.Time         `json:"cancelled_at,omitempty"`
	CancelReason *string            `json:"cancel_reason,omitempty"`
	Targets      []AssignmentTarget `json:"targets"`
	Counters     AssignmentCounters `json:"counters"`
}

// FromDomain заполняет Assignment доменным назначением; createdBy — резолвится вызывающей
// стороной батчем (в карточке — из того же батча пользователей, что и участники).
func (a *Assignment) FromDomain(
	assignment domain.Assignment,
	createdBy domain.User,
	targets []domain.AssignmentTarget,
	counters domain.AssignmentCounters,
) {
	a.ID = assignment.ID
	a.VideoID = assignment.VideoID
	a.VideoName = assignment.VideoName
	a.GroupID = assignment.GroupID
	a.GroupName = assignment.GroupName
	a.CreatedBy.FromDomain(createdBy)
	a.CreatedAt = assignment.CreatedAt
	a.DueMode = string(assignment.DueMode)
	a.DueAt = assignment.DueAt
	a.DueDays = assignment.DueDays
	a.Comment = assignment.Comment
	a.Status = string(assignment.Status)
	a.CancelledAt = assignment.CancelledAt
	if assignment.CancelReason != nil {
		reason := string(*assignment.CancelReason)
		a.CancelReason = &reason
	}

	a.Targets = make([]AssignmentTarget, len(targets))
	for i, t := range targets {
		a.Targets[i].FromDomain(t)
	}

	a.Counters.FromDomain(counters)
}

// AssignmentParticipant — персональная запись участника назначения (§5 контракта эпика Э3,
// пожелание фронта В-55: добавлено CancelledAt). IsOverdue/CompletedLate — вычисляются от
// времени ответа, не хранятся (Э3-Т4).
type AssignmentParticipant struct {
	User          AssignmentUser `json:"user"`
	Status        string         `json:"status"`
	Source        string         `json:"source"`
	SourceGroupID *uuid.UUID     `json:"source_group_id,omitempty"`
	EnrolledAt    time.Time      `json:"enrolled_at"`
	DueAt         time.Time      `json:"due_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	CancelledAt   *time.Time     `json:"cancelled_at,omitempty"`
	CoveragePct   int            `json:"coverage_pct"`
	IsOverdue     bool           `json:"is_overdue"`
	CompletedLate bool           `json:"completed_late"`
	HasAccess     bool           `json:"has_access"`
	CancelReason  *string        `json:"cancel_reason,omitempty"`
}

// FromDomain заполняет AssignmentParticipant доменной карточкой участника (§4 дизайна эпика
// Э3, ParticipantDetails).
func (p *AssignmentParticipant) FromDomain(details domain.ParticipantDetails) {
	participant := details.Participant

	p.User.FromDomain(details.User)
	p.Status = string(participant.Status)
	p.Source = string(participant.Source)
	p.SourceGroupID = participant.SourceGroupID
	p.EnrolledAt = participant.EnrolledAt
	p.DueAt = participant.DueAt
	p.CompletedAt = participant.CompletedAt
	p.CancelledAt = participant.CancelledAt
	p.CoveragePct = details.CoveragePct
	p.IsOverdue = participant.IsOverdue(time.Now())
	p.CompletedLate = participant.CompletedLate()
	p.HasAccess = details.HasAccess
	if participant.CancelReason != nil {
		reason := string(*participant.CancelReason)
		p.CancelReason = &reason
	}
}

// AssignmentEvent — запись журнала назначения (§5 контракта эпика Э3, Э3-Т32). Actor — nil,
// если событие сгенерировано системой (heartbeat, каскад).
type AssignmentEvent struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	UserID    *uuid.UUID      `json:"user_id,omitempty"`
	Actor     *AssignmentUser `json:"actor,omitempty"`
	Payload   json.RawMessage `json:"payload"           swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
}

// FromDomain заполняет AssignmentEvent доменной записью журнала.
func (e *AssignmentEvent) FromDomain(details domain.EventDetails) {
	e.ID = details.Event.ID
	e.Type = string(details.Event.Type)
	e.UserID = details.Event.UserID
	e.Payload = details.Event.Payload
	e.CreatedAt = details.Event.CreatedAt

	if details.Actor != nil {
		actor := AssignmentUser{}
		actor.FromDomain(*details.Actor)
		e.Actor = &actor
	}
}

// AssignmentDetails — карточка назначения целиком: участники и журнал (§5 контракта эпика Э3,
// ручка GET .../assignments/{id}).
type AssignmentDetails struct {
	Assignment

	Participants []AssignmentParticipant `json:"participants"`
	Events       []AssignmentEvent       `json:"events"`
}

// FromDomain заполняет AssignmentDetails доменной карточкой назначения.
func (d *AssignmentDetails) FromDomain(details domain.AssignmentDetails) {
	d.Assignment.FromDomain(details.Assignment, details.CreatedByUser, details.Targets, details.Counters)

	d.Participants = make([]AssignmentParticipant, len(details.Participants))
	for i, p := range details.Participants {
		d.Participants[i].FromDomain(p)
	}

	d.Events = make([]AssignmentEvent, len(details.Events))
	for i, e := range details.Events {
		d.Events[i].FromDomain(e)
	}
}

// RejectedTarget — пользователь, которого не удалось включить в назначение при создании (В-4,
// §5 контракта эпика Э3).
type RejectedTarget struct {
	UserID  uuid.UUID `json:"user_id"`
	Name    string    `json:"name"`
	Surname string    `json:"surname"`
	Email   string    `json:"email"`
	Reason  string    `json:"reason"`
}

// FromDomain заполняет RejectedTarget доменной отклонённой целью.
func (r *RejectedTarget) FromDomain(target domain.RejectedTarget) {
	r.UserID = target.UserID
	r.Name = target.Name
	r.Surname = target.Surname
	r.Email = target.Email
	r.Reason = string(target.Reason)
}

// CreateAssignmentRequest — тело запроса создания назначения (§5 контракта эпика Э3).
type CreateAssignmentRequest struct {
	VideoID uuid.UUID   `json:"video_id" binding:"required"`
	Users   []uuid.UUID `json:"users"`
	Groups  []uuid.UUID `json:"groups"`
	DueMode string      `json:"due_mode" binding:"required,oneof=date days"`
	DueAt   *time.Time  `json:"due_at"`
	DueDays *int        `json:"due_days"`
	Comment string      `json:"comment"  binding:"max=500"`
}

// ToDomain конвертирует запрос создания назначения в domain.CreateAssignment.
func (r *CreateAssignmentRequest) ToDomain() domain.CreateAssignment {
	return domain.CreateAssignment{
		VideoID: r.VideoID,
		Users:   r.Users,
		Groups:  r.Groups,
		DueMode: domain.AssignmentDueMode(r.DueMode),
		DueAt:   r.DueAt,
		DueDays: r.DueDays,
		Comment: r.Comment,
	}
}

// CreateAssignmentResponse — ответ на создание назначения: назначение и список целей, не
// включённых в него (В-4, §5 контракта эпика Э3).
type CreateAssignmentResponse struct {
	Assignment AssignmentDetails `json:"assignment"`
	Rejected   []RejectedTarget  `json:"rejected"`
}

// GetAssignmentResponse — ответ на получение карточки назначения (§5 контракта эпика Э3).
type GetAssignmentResponse struct {
	Assignment AssignmentDetails `json:"assignment"`
}

// MyAssignmentVideo — видео назначения в контексте «моих назначений»: текущее состояние или
// снимок, если видео удалено (§5 контракта эпика Э3).
type MyAssignmentVideo struct {
	ID         *uuid.UUID `json:"id"`
	Name       string     `json:"name"`
	GroupID    *uuid.UUID `json:"group_id"`
	GroupName  string     `json:"group_name"`
	Status     *uint      `json:"status,omitempty"`
	StatusName string     `json:"status_name,omitempty"`
	DurationMs *int64     `json:"duration_ms,omitempty"`
	IsDeleted  bool       `json:"is_deleted"`
}

// MyAssignment — назначение в разрезе одного участника для ручки GET me/assignments (§5
// контракта эпика Э3, пожелание фронта В-55: добавлено CancelledAt).
type MyAssignment struct {
	ID            uuid.UUID         `json:"id"`
	Video         MyAssignmentVideo `json:"video"`
	DueAt         time.Time         `json:"due_at"`
	Status        string            `json:"status"`
	IsOverdue     bool              `json:"is_overdue"`
	CoveragePct   int               `json:"coverage_pct"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	CancelledAt   *time.Time        `json:"cancelled_at,omitempty"`
	EnrolledAt    time.Time         `json:"enrolled_at"`
	Source        string            `json:"source"`
	AssignedBy    AssignmentUser    `json:"assigned_by"`
	Comment       string            `json:"comment"`
	CompletedLate bool              `json:"completed_late"`
}

// FromDomain заполняет MyAssignment доменным элементом «моих назначений».
func (m *MyAssignment) FromDomain(item domain.MyAssignment) {
	participant := item.Participant
	assignment := item.Assignment

	m.ID = assignment.ID
	m.Video = myAssignmentVideoFromDomain(assignment, item.Video)
	m.DueAt = participant.DueAt
	m.Status = string(participant.Status)
	m.IsOverdue = participant.IsOverdue(time.Now())
	m.CoveragePct = item.CoveragePct
	m.CompletedAt = participant.CompletedAt
	m.CancelledAt = participant.CancelledAt
	m.EnrolledAt = participant.EnrolledAt
	m.Source = string(participant.Source)
	m.AssignedBy.FromDomain(item.AssignedBy)
	m.Comment = assignment.Comment
	m.CompletedLate = participant.CompletedLate()
}

// myAssignmentVideoFromDomain собирает представление видео назначения: текущее состояние,
// если видео существует, иначе — снимок из самого назначения (Э3-Т7).
func myAssignmentVideoFromDomain(assignment domain.Assignment, video *domain.Video) MyAssignmentVideo {
	v := MyAssignmentVideo{
		GroupID: assignment.GroupID, GroupName: assignment.GroupName,
		Name: assignment.VideoName, IsDeleted: video == nil,
	}

	if video != nil {
		id := video.ID
		status := uint(video.Status)
		v.ID = &id
		v.Name = video.Name
		v.Status = &status
		v.StatusName = video.Status.String()
		v.DurationMs = video.DurationMs
	}

	return v
}

// MyAssignmentsResponse — ответ на GET me/assignments: назначения пользователя и сводка по
// активным/просроченным (§5 контракта эпика Э3).
type MyAssignmentsResponse struct {
	Assignments  []MyAssignment `json:"assignments"`
	ActiveCount  int            `json:"active_count"`
	OverdueCount int            `json:"overdue_count"`
}

// FromDomain заполняет MyAssignmentsResponse доменным списком «моих назначений» — счётчики
// active/overdue считаются от уже вычисленных полей элементов (Status/IsOverdue).
func (r *MyAssignmentsResponse) FromDomain(items []domain.MyAssignment) {
	r.Assignments = make([]MyAssignment, len(items))
	for i, item := range items {
		r.Assignments[i].FromDomain(item)

		active := r.Assignments[i].Status == string(domain.AssignmentParticipantStatusAssigned) ||
			r.Assignments[i].Status == string(domain.AssignmentParticipantStatusInProgress)
		if !active {
			continue
		}

		r.ActiveCount++
		if r.Assignments[i].IsOverdue {
			r.OverdueCount++
		}
	}
}
