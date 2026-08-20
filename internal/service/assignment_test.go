package service_test

import (
	"context"
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// assignmentMocks собирает моки, используемые AssignmentService.
type assignmentMocks struct {
	Assignment   *repository_mocks.AssignmentMock
	Targets      *repository_mocks.AssignmentTargetMock
	Participants *repository_mocks.AssignmentParticipantMock
	Events       *repository_mocks.AssignmentEventMock
	Progress     *repository_mocks.WatchProgressMock
	Video        *repository_mocks.VideoMock
	GroupMembers *repository_mocks.GroupMemberMock
	Access       *service_mocks.AccessMock
	User         *service_mocks.UserMock
	UserGroup    *service_mocks.UserGroupMock
	AccountRole  *service_mocks.AccountRoleMock
}

func newAssignmentMocks(mc *minimock.Controller) assignmentMocks {
	return assignmentMocks{
		Assignment:   repository_mocks.NewAssignmentMock(mc),
		Targets:      repository_mocks.NewAssignmentTargetMock(mc),
		Participants: repository_mocks.NewAssignmentParticipantMock(mc),
		Events:       repository_mocks.NewAssignmentEventMock(mc),
		Progress:     repository_mocks.NewWatchProgressMock(mc),
		Video:        repository_mocks.NewVideoMock(mc),
		GroupMembers: repository_mocks.NewGroupMemberMock(mc),
		Access:       service_mocks.NewAccessMock(mc),
		User:         service_mocks.NewUserMock(mc),
		UserGroup:    service_mocks.NewUserGroupMock(mc),
		AccountRole:  service_mocks.NewAccountRoleMock(mc),
	}
}

func newAssignmentService(m assignmentMocks, cfg config.VideoConfig, now time.Time) *service.AssignmentService {
	svc := &service.Service{
		Access: m.Access, User: m.User, UserGroup: m.UserGroup, AccountRole: m.AccountRole,
	}
	return service.NewAssignmentService(
		m.Assignment, m.Targets, m.Participants, m.Events, m.Progress, m.Video, m.GroupMembers,
		svc, cfg,
		service.WithAssignmentNow(func() time.Time { return now }),
	)
}

// assignmentFixture — общие идентификаторы, конфиг и видео для тестов AssignmentService.
type assignmentFixture struct {
	AccountID    uuid.UUID
	GroupID      uuid.UUID
	VideoID      uuid.UUID
	InitiatorID  uuid.UUID
	AssignmentID uuid.UUID
	Now          time.Time
	Cfg          config.VideoConfig
	Video        domain.Video
}

func newAssignmentFixture() assignmentFixture {
	videoID := uuid.New()
	groupID := uuid.New()

	return assignmentFixture{
		AccountID:    uuid.New(),
		GroupID:      groupID,
		VideoID:      videoID,
		InitiatorID:  uuid.New(),
		AssignmentID: uuid.New(),
		Now:          time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC),
		Cfg:          config.VideoConfig{WatchCompletionThreshold: 0.95, WatchHeartbeatInterval: 10 * time.Second},
		Video: domain.Video{
			ID: videoID, GroupID: groupID, Name: "Регламент", Status: domain.VideoStatusReady,
		},
	}
}

func validCreateInput(f assignmentFixture, users ...uuid.UUID) domain.CreateAssignment {
	dueAt := f.Now.Add(7 * 24 * time.Hour)
	return domain.CreateAssignment{
		VideoID: f.VideoID, Users: users, DueMode: domain.AssignmentDueModeDate, DueAt: &dueAt,
	}
}

func TestService_AssignmentCreate_ForbiddenWithoutManagePermission(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID, f.GroupID).
		Return(service.ErrForbidden)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, validCreateInput(f, uuid.New()))

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_AssignmentCreate_VideoNotFound(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(nil, repository.ErrNotFound)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, validCreateInput(f, uuid.New()))

	require.ErrorIs(t, err, repository.ErrNotFound)
}

func TestService_AssignmentCreate_VideoNotAssignable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status domain.VideoStatus
	}{
		{name: "uploading", status: domain.VideoStatusUploading},
		{name: "failed", status: domain.VideoStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newAssignmentFixture()
			f.Video.Status = tt.status
			mc := minimock.NewController(t)
			m := newAssignmentMocks(mc)

			m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(&f.Video, nil)
			m.Access.CanManageAssignmentsMock.Return(nil)

			svc := newAssignmentService(m, f.Cfg, f.Now)
			_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, validCreateInput(f, uuid.New()))

			require.ErrorIs(t, err, service.ErrVideoNotAssignable)
		})
	}
}

func TestService_AssignmentCreate_ValidatesDue(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	pastDueAt := f.Now.Add(-time.Hour)
	futureDueAt := f.Now.Add(time.Hour)
	zeroDays := 0
	tooManyDays := 3651
	okDays := 7

	tests := []struct {
		name    string
		in      domain.CreateAssignment
		wantErr error
	}{
		{
			name: "date mode without due_at",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Users:   []uuid.UUID{uuid.New()},
				DueMode: domain.AssignmentDueModeDate,
			},
			wantErr: service.ErrDueAtInvalid,
		},
		{
			name: "date mode in the past",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Users:   []uuid.UUID{uuid.New()},
				DueMode: domain.AssignmentDueModeDate,
				DueAt:   &pastDueAt,
			},
			wantErr: service.ErrDueAtInvalid,
		},
		{
			name: "days mode without due_days",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Users:   []uuid.UUID{uuid.New()},
				DueMode: domain.AssignmentDueModeDays,
			},
			wantErr: service.ErrDueDaysInvalid,
		},
		{
			name: "days mode zero",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Users:   []uuid.UUID{uuid.New()},
				DueMode: domain.AssignmentDueModeDays,
				DueDays: &zeroDays,
			},
			wantErr: service.ErrDueDaysInvalid,
		},
		{
			name: "days mode too large",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Users:   []uuid.UUID{uuid.New()},
				DueMode: domain.AssignmentDueModeDays,
				DueDays: &tooManyDays,
			},
			wantErr: service.ErrDueDaysInvalid,
		},
		{
			name: "date mode valid does not fail due validation",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Groups:  []uuid.UUID{f.GroupID},
				DueMode: domain.AssignmentDueModeDate,
				DueAt:   &futureDueAt,
			},
		},
		{
			name: "days mode valid does not fail due validation",
			in: domain.CreateAssignment{
				VideoID: f.VideoID,
				Groups:  []uuid.UUID{f.GroupID},
				DueMode: domain.AssignmentDueModeDays,
				DueDays: &okDays,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := minimock.NewController(t)
			m := newAssignmentMocks(mc)

			m.Video.SelectMock.Return(&f.Video, nil)
			m.Access.CanManageAssignmentsMock.Return(nil)
			if tt.wantErr == nil {
				// Валидный срок доходит до проверки целей — группа=video.GroupID валидна,
				// но кандидатов нет (пустая группа), поэтому дальше сервис завершится без
				// ошибки уже на этапе создания записей — регистрируем нужные моки.
				m.UserGroup.GetByIDMock.Return(
					[]domain.UserGroup{{ID: f.GroupID, Name: "G", AccountID: f.AccountID}},
					nil,
				)
				m.GroupMembers.SelectByGroupIDMock.Return(nil, nil)
				m.Assignment.InsertMock.Return(domain.Assignment{ID: f.AssignmentID, AccountID: f.AccountID}, nil)
				m.Targets.InsertBatchMock.Return(nil, nil)
				m.Events.InsertBatchMock.Return(nil, nil)
				m.Assignment.SelectByIDMock.Return(
					domain.Assignment{ID: f.AssignmentID, AccountID: f.AccountID, CreatedBy: f.InitiatorID},
					nil,
				)
				m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
				m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
				m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
				m.Events.SelectByAssignmentIDMock.Return(nil, nil)
				m.User.GetByIDsMock.Return(nil, nil)
			}

			svc := newAssignmentService(m, f.Cfg, f.Now)
			_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, tt.in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_AssignmentCreate_TargetsEmpty(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Video.SelectMock.Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	in := domain.CreateAssignment{
		VideoID: f.VideoID,
		DueMode: domain.AssignmentDueModeDate,
		DueAt:   ptrTime(f.Now.Add(time.Hour)),
	}
	_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, in)

	require.ErrorIs(t, err, service.ErrTargetsEmpty)
}

func TestService_AssignmentCreate_TargetGroupMustMatchVideoGroup(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Video.SelectMock.Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	in := domain.CreateAssignment{
		VideoID: f.VideoID, Groups: []uuid.UUID{uuid.New()},
		DueMode: domain.AssignmentDueModeDate, DueAt: ptrTime(f.Now.Add(time.Hour)),
	}
	_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, in)

	require.ErrorIs(t, err, service.ErrTargetGroupInvalid)
}

// TestService_AssignmentCreate_RejectsTargetsByReasonAndAcceptsValid проверяет раскрытие целей
// (§4 дизайна эпика Э3, шаг 5, В-4): не в аккаунте / деактивирован / нет доступа — rejected, не
// ошибка; принятый кандидат зачисляется участником.
func TestService_AssignmentCreate_RejectsTargetsByReasonAndAcceptsValid(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	roleOK := uuid.New()
	roleOtherAccount := uuid.New()
	otherAccountID := uuid.New()

	missingUserID := uuid.New() // не будет найден в GetByIDs — not_in_account
	wrongAccountUser := domain.User{ID: uuid.New(), Name: "Chужой", RoleID: roleOtherAccount}
	inactiveUser := domain.User{
		ID: uuid.New(), Name: "Неактивный", RoleID: roleOK, DeactivatedAt: ptrTime(f.Now.Add(-time.Hour)),
	}
	noAccessUser := domain.User{ID: uuid.New(), Name: "БезДоступа", RoleID: roleOK}
	okUser := domain.User{ID: uuid.New(), Name: "Иван", Surname: "Иванов", Email: "ivan@example.com", RoleID: roleOK}
	knownUsers := map[uuid.UUID]domain.User{
		wrongAccountUser.ID: wrongAccountUser,
		inactiveUser.ID:     inactiveUser,
		noAccessUser.ID:     noAccessUser,
		okUser.ID:           okUser,
		f.InitiatorID:       {ID: f.InitiatorID, Name: "Создатель"},
	}

	m.Video.SelectMock.Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.UserGroup.GetByIDMock.Return([]domain.UserGroup{{ID: f.GroupID, Name: "Продажи", AccountID: f.AccountID}}, nil)

	m.User.GetByIDsMock.Set(func(_ context.Context, ids []uuid.UUID) ([]domain.User, error) {
		result := make([]domain.User, 0, len(ids))
		for _, id := range ids {
			if u, ok := knownUsers[id]; ok {
				result = append(result, u)
			}
		}
		return result, nil
	})
	m.AccountRole.GetByIDMock.Set(func(_ context.Context, roleIDs ...uuid.UUID) ([]domain.AccountRole, error) {
		roles := make([]domain.AccountRole, 0, len(roleIDs))
		for _, id := range roleIDs {
			switch id {
			case roleOK:
				roles = append(roles, domain.AccountRole{ID: roleOK, AccountID: f.AccountID})
			case roleOtherAccount:
				roles = append(roles, domain.AccountRole{ID: roleOtherAccount, AccountID: otherAccountID})
			}
		}
		return roles, nil
	})
	m.Access.CanWatchVideoMock.Set(func(_ context.Context, _, userID, _ uuid.UUID) bool {
		return userID == okUser.ID
	})

	var insertedParticipants []domain.AssignmentParticipant
	m.Participants.InsertBatchMock.Set(
		func(_ context.Context, p []domain.AssignmentParticipant) ([]domain.AssignmentParticipant, error) {
			insertedParticipants = p
			return p, nil
		},
	)

	var insertedEvents []domain.AssignmentEvent
	m.Events.InsertBatchMock.Set(func(_ context.Context, e []domain.AssignmentEvent) ([]domain.AssignmentEvent, error) {
		insertedEvents = e
		return e, nil
	})

	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	m.Targets.InsertBatchMock.Return(nil, nil)
	m.Assignment.InsertMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, VideoName: f.Video.Name,
		GroupID: &f.GroupID, GroupName: "Продажи", CreatedBy: f.InitiatorID, Status: domain.AssignmentStatusActive,
	}, nil)

	// Хвост Create — сборка ответа через Get, инициатор совпадает с автором (проверка прав
	// пропускается без ManagedAssignmentGroups).
	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID,
		CreatedBy: f.InitiatorID, Status: domain.AssignmentStatusActive,
	}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	in := validCreateInput(f, missingUserID, wrongAccountUser.ID, inactiveUser.ID, noAccessUser.ID, okUser.ID)
	_, rejected, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, in)

	require.NoError(t, err)
	require.Len(t, rejected, 4)

	reasonByUser := make(map[uuid.UUID]domain.RejectedReason, len(rejected))
	for _, r := range rejected {
		reasonByUser[r.UserID] = r.Reason
	}
	require.Equal(t, domain.RejectedReasonNotInAccount, reasonByUser[missingUserID])
	require.Equal(t, domain.RejectedReasonNotInAccount, reasonByUser[wrongAccountUser.ID])
	require.Equal(t, domain.RejectedReasonInactive, reasonByUser[inactiveUser.ID])
	require.Equal(t, domain.RejectedReasonNoAccess, reasonByUser[noAccessUser.ID])

	require.Len(t, insertedParticipants, 1)
	require.Equal(t, okUser.ID, insertedParticipants[0].UserID)
	require.Equal(t, domain.AssignmentParticipantSourcePersonal, insertedParticipants[0].Source)
	require.Equal(t, domain.AssignmentParticipantStatusAssigned, insertedParticipants[0].Status)

	// created + participant_enrolled(1) + participant_rejected(4).
	require.Len(t, insertedEvents, 6)
}

// TestService_AssignmentCreate_CompletesParticipantAlreadyWatched проверяет В-11: если
// покрытие уже достигло порога до назначения, участник сразу completed с completed_at,
// равным моменту достижения порога.
func TestService_AssignmentCreate_CompletesParticipantAlreadyWatched(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	f.Video.DurationMs = ptrInt64(100000)
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	user := domain.User{ID: uuid.New(), Name: "Иван", RoleID: uuid.New()}
	role := domain.AccountRole{ID: user.RoleID, AccountID: f.AccountID}
	thresholdReachedAt := f.Now.Add(-time.Hour)

	m.Video.SelectMock.Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.UserGroup.GetByIDMock.Return([]domain.UserGroup{{ID: f.GroupID, Name: "Продажи", AccountID: f.AccountID}}, nil)
	m.User.GetByIDsMock.Return([]domain.User{user}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{role}, nil)
	m.Access.CanWatchVideoMock.Return(true)
	m.Progress.SelectByVideoIDsMock.Return([]domain.WatchProgress{
		{UserID: user.ID, VideoID: f.VideoID, CoveredMs: 98000, ThresholdReachedAt: &thresholdReachedAt},
	}, nil)
	m.Targets.InsertBatchMock.Return(nil, nil)
	m.Assignment.InsertMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID,
		CreatedBy: f.InitiatorID, Status: domain.AssignmentStatusActive,
	}, nil)

	var insertedParticipants []domain.AssignmentParticipant
	m.Participants.InsertBatchMock.Set(
		func(_ context.Context, p []domain.AssignmentParticipant) ([]domain.AssignmentParticipant, error) {
			insertedParticipants = p
			return p, nil
		},
	)
	m.Events.InsertBatchMock.Return(nil, nil)

	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID, CreatedBy: f.InitiatorID,
	}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, rejected, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, validCreateInput(f, user.ID))

	require.NoError(t, err)
	require.Empty(t, rejected)
	require.Len(t, insertedParticipants, 1)
	require.Equal(t, domain.AssignmentParticipantStatusCompleted, insertedParticipants[0].Status)
	require.NotNil(t, insertedParticipants[0].CompletedAt)
	require.Equal(t, thresholdReachedAt, *insertedParticipants[0].CompletedAt)
	require.Equal(t, 98, *insertedParticipants[0].CompletedCoveragePct)
}

// TestService_AssignmentCreate_MarksInProgressWhenPartialCoverage проверяет, что частичный
// прогресс без достижения порога даёт статус in_progress, а не completed/assigned.
func TestService_AssignmentCreate_MarksInProgressWhenPartialCoverage(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	f.Video.DurationMs = ptrInt64(100000)
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	user := domain.User{ID: uuid.New(), Name: "Иван", RoleID: uuid.New()}
	role := domain.AccountRole{ID: user.RoleID, AccountID: f.AccountID}

	m.Video.SelectMock.Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.UserGroup.GetByIDMock.Return([]domain.UserGroup{{ID: f.GroupID, Name: "Продажи", AccountID: f.AccountID}}, nil)
	m.User.GetByIDsMock.Return([]domain.User{user}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{role}, nil)
	m.Access.CanWatchVideoMock.Return(true)
	m.Progress.SelectByVideoIDsMock.Return([]domain.WatchProgress{
		{UserID: user.ID, VideoID: f.VideoID, CoveredMs: 5000},
	}, nil)
	m.Targets.InsertBatchMock.Return(nil, nil)
	m.Assignment.InsertMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID, CreatedBy: f.InitiatorID,
	}, nil)

	var insertedParticipants []domain.AssignmentParticipant
	m.Participants.InsertBatchMock.Set(
		func(_ context.Context, p []domain.AssignmentParticipant) ([]domain.AssignmentParticipant, error) {
			insertedParticipants = p
			return p, nil
		},
	)
	m.Events.InsertBatchMock.Return(nil, nil)
	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID, CreatedBy: f.InitiatorID,
	}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, _, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, validCreateInput(f, user.ID))

	require.NoError(t, err)
	require.Len(t, insertedParticipants, 1)
	require.Equal(t, domain.AssignmentParticipantStatusInProgress, insertedParticipants[0].Status)
}

// TestService_AssignmentCreate_GroupTargetEnrollsMembers проверяет раскрытие цели-группы: её
// текущие участники становятся кандидатами с source=group.
func TestService_AssignmentCreate_GroupTargetEnrollsMembers(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	member := domain.User{ID: uuid.New(), Name: "Пётр", RoleID: uuid.New()}
	role := domain.AccountRole{ID: member.RoleID, AccountID: f.AccountID}

	m.Video.SelectMock.Return(&f.Video, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.UserGroup.GetByIDMock.Return([]domain.UserGroup{{ID: f.GroupID, Name: "Продажи", AccountID: f.AccountID}}, nil)
	m.GroupMembers.SelectByGroupIDMock.Expect(minimock.AnyContext, f.GroupID).
		Return([]domain.GroupMember{{GroupID: f.GroupID, UserID: member.ID}}, nil)
	m.User.GetByIDsMock.Return([]domain.User{member}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{role}, nil)
	m.Access.CanWatchVideoMock.Return(true)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	m.Targets.InsertBatchMock.Return(nil, nil)
	m.Assignment.InsertMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID, CreatedBy: f.InitiatorID,
	}, nil)

	var insertedParticipants []domain.AssignmentParticipant
	m.Participants.InsertBatchMock.Set(
		func(_ context.Context, p []domain.AssignmentParticipant) ([]domain.AssignmentParticipant, error) {
			insertedParticipants = p
			return p, nil
		},
	)
	m.Events.InsertBatchMock.Return(nil, nil)
	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID, CreatedBy: f.InitiatorID,
	}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	in := domain.CreateAssignment{
		VideoID: f.VideoID, Groups: []uuid.UUID{f.GroupID},
		DueMode: domain.AssignmentDueModeDate, DueAt: ptrTime(f.Now.Add(time.Hour)),
	}
	_, rejected, err := svc.Create(t.Context(), f.AccountID, f.InitiatorID, in)

	require.NoError(t, err)
	require.Empty(t, rejected)
	require.Len(t, insertedParticipants, 1)
	require.Equal(t, domain.AssignmentParticipantSourceGroup, insertedParticipants[0].Source)
	require.Equal(t, f.GroupID, *insertedParticipants[0].SourceGroupID)
}

func TestService_AssignmentGet_NotFoundWrongAccount(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Expect(minimock.AnyContext, f.AssignmentID).
		Return(domain.Assignment{ID: f.AssignmentID, AccountID: uuid.New()}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, err := svc.Get(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestService_AssignmentGet_AllowedForAuthorWithoutManagePermission(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, CreatedBy: f.InitiatorID,
	}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)
	m.User.GetByIDsMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	details, err := svc.Get(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

	require.NoError(t, err)
	require.Equal(t, f.AssignmentID, details.Assignment.ID)
	// ManagedAssignmentGroups не должен вызываться — автор видит своё назначение без него.
	require.Zero(t, m.Access.ManagedAssignmentGroupsAfterCounter())
}

func TestService_AssignmentGet_ForbiddenWithoutAccessInArea(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	otherInitiator := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, CreatedBy: f.InitiatorID, GroupID: &f.GroupID,
	}, nil)
	m.Access.ManagedAssignmentGroupsMock.Expect(minimock.AnyContext, f.AccountID, otherInitiator).
		Return(false, nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, err := svc.Get(t.Context(), f.AccountID, otherInitiator, f.AssignmentID)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_AssignmentGet_AllowedForAccountWideManager(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	manager := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Return(domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, CreatedBy: f.InitiatorID, GroupID: &f.GroupID,
	}, nil)
	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)
	m.User.GetByIDsMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	details, err := svc.Get(t.Context(), f.AccountID, manager, f.AssignmentID)

	require.NoError(t, err)
	require.Equal(t, f.AssignmentID, details.Assignment.ID)
}

func TestService_AssignmentListMine_EmptyWhenNoParticipants(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	userID := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Participants.SelectByUserIDMock.Expect(minimock.AnyContext, userID).Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.ListMine(t.Context(), userID)

	require.NoError(t, err)
	require.Empty(t, items)
}

func TestService_AssignmentListMine_MarksDeletedVideoWithSnapshot(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	userID := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	participant := domain.AssignmentParticipant{
		AssignmentID: f.AssignmentID, UserID: userID, Status: domain.AssignmentParticipantStatusCancelled,
		DueAt: f.Now.Add(time.Hour),
	}
	assignment := domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: nil, VideoName: "Удалённое видео",
		GroupName: "Продажи", CreatedBy: f.InitiatorID,
	}

	m.Participants.SelectByUserIDMock.Return([]domain.AssignmentParticipant{participant}, nil)
	m.Assignment.SelectByIDsMock.Return([]domain.Assignment{assignment}, nil)
	m.Video.SelectByIDsMock.Return(nil, nil)
	m.User.GetByIDsMock.Return([]domain.User{{ID: f.InitiatorID, Name: "Автор"}}, nil)
	m.Progress.SelectByUserAndVideoIDsMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.ListMine(t.Context(), userID)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Nil(t, items[0].Video)
	require.Equal(t, "Удалённое видео", items[0].Assignment.VideoName)
	require.Equal(t, "Автор", items[0].AssignedBy.Name)
}

func TestService_AssignmentListMine_UsesCompletedCoverageForCompletedParticipant(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	userID := uuid.New()
	coverage := 97
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	participant := domain.AssignmentParticipant{
		AssignmentID: f.AssignmentID, UserID: userID, Status: domain.AssignmentParticipantStatusCompleted,
		DueAt: f.Now.Add(time.Hour), CompletedCoveragePct: &coverage,
	}
	assignment := domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, CreatedBy: f.InitiatorID,
	}

	m.Participants.SelectByUserIDMock.Return([]domain.AssignmentParticipant{participant}, nil)
	m.Assignment.SelectByIDsMock.Return([]domain.Assignment{assignment}, nil)
	m.Video.SelectByIDsMock.Return([]domain.Video{f.Video}, nil)
	m.User.GetByIDsMock.Return([]domain.User{{ID: f.InitiatorID, Name: "Автор"}}, nil)
	// Прогресс сейчас может отличаться от зафиксированного на момент завершения — используется
	// зафиксированный процент, а не текущий прогресс.
	m.Progress.SelectByUserAndVideoIDsMock.Return([]domain.WatchProgress{
		{UserID: userID, VideoID: f.VideoID, CoveredMs: 1},
	}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.ListMine(t.Context(), userID)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, coverage, items[0].CoveragePct)
}

func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt64(v int64) *int64        { return &v }
