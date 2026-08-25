package service_test

import (
	"context"
	"encoding/json"
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
	Chapters     *repository_mocks.ChapterMock
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
		Chapters:     repository_mocks.NewChapterMock(mc),
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
		m.Assignment, m.Targets, m.Participants, m.Events, m.Progress, m.Video, m.GroupMembers, m.Chapters,
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

// TestService_AssignmentGet_ChapterProgressForParticipants проверяет КП-4 (§6 дизайна эпика
// Э4): карточка назначения отдаёт по каждому участнику «глав пройдено X из Y» и раскрываемую
// детализацию, посчитанную по текущему покрытию (в т.ч. участник, ни разу не открывавший
// видео — все главы "не просмотрены"). Границы и покрытие запрашиваются одним обращением к
// каждому из двух методов репозитория глав независимо от числа участников — не по одному на
// участника (Н1): проверяется счётчиком вызовов мока.
func TestService_AssignmentGet_ChapterProgressForParticipants(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	f.Video.DurationMs = ptrInt64(10000)
	watchedUserID := uuid.New()
	freshUserID := uuid.New()
	chapter1ID := uuid.New()
	chapter2ID := uuid.New()

	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID,
		CreatedBy: f.InitiatorID,
	}
	m.Assignment.SelectByIDMock.Return(assignment, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return([]domain.AssignmentParticipant{
		{AssignmentID: f.AssignmentID, UserID: watchedUserID, Status: domain.AssignmentParticipantStatusInProgress},
		{AssignmentID: f.AssignmentID, UserID: freshUserID, Status: domain.AssignmentParticipantStatusAssigned},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: watchedUserID, Name: "Смотрел"}, {ID: freshUserID, Name: "Не открывал"},
	}, nil)
	m.Video.SelectMock.Return(&f.Video, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	m.Access.CanWatchVideoMock.Return(true)

	bounds := []domain.ChapterBound{
		{Chapter: domain.Chapter{ID: chapter1ID, VideoID: f.VideoID, Name: "Глава 1", StartMs: 0}, EndMs: 5000},
		{Chapter: domain.Chapter{ID: chapter2ID, VideoID: f.VideoID, Name: "Глава 2", StartMs: 5000}, EndMs: 10000},
	}
	m.Chapters.SelectBoundsByVideoIDMock.Expect(minimock.AnyContext, f.VideoID, int64(10000)).Return(bounds, nil)
	m.Chapters.SelectProgressByVideoAndUsersMock.
		Expect(minimock.AnyContext, f.VideoID, []uuid.UUID{watchedUserID, freshUserID}, int64(10000)).
		Return([]domain.ChapterUserProgress{
			{
				ChapterProgress: domain.ChapterProgress{ChapterBound: bounds[0], CoveredMs: 5000},
				UserID:          watchedUserID,
			},
			{
				ChapterProgress: domain.ChapterProgress{ChapterBound: bounds[1], CoveredMs: 2000},
				UserID:          watchedUserID,
			},
		}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	details, err := svc.Get(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

	require.NoError(t, err)
	require.Len(t, details.Participants, 2)

	byUser := make(map[uuid.UUID]domain.ParticipantDetails, len(details.Participants))
	for _, p := range details.Participants {
		byUser[p.Participant.UserID] = p
	}

	watched := byUser[watchedUserID].ChapterProgress
	require.NotNil(t, watched)
	require.Equal(t, 2, watched.Total)
	require.Equal(t, 1, watched.Completed)
	require.Equal(t, []domain.ParticipantChapterStatus{
		{Name: "Глава 1", CoveragePct: 100, Status: domain.ChapterStatusDone},
		{Name: "Глава 2", CoveragePct: 40, Status: domain.ChapterStatusPartial},
	}, watched.Chapters)

	fresh := byUser[freshUserID].ChapterProgress
	require.NotNil(t, fresh)
	require.Equal(t, 2, fresh.Total)
	require.Equal(t, 0, fresh.Completed)
	require.Equal(t, []domain.ParticipantChapterStatus{
		{Name: "Глава 1", CoveragePct: 0, Status: domain.ChapterStatusNotStarted},
		{Name: "Глава 2", CoveragePct: 0, Status: domain.ChapterStatusNotStarted},
	}, fresh.Chapters)

	// Ровно один запрос на карточку на каждый из двух методов — не по одному на участника (Н1).
	require.EqualValues(t, 1, m.Chapters.SelectBoundsByVideoIDAfterCounter())
	require.EqualValues(t, 1, m.Chapters.SelectProgressByVideoAndUsersAfterCounter())
}

// TestService_AssignmentGet_ChapterProgressAbsentWithoutChaptersOrVideo проверяет §6 дизайна
// эпика Э4: сводка по главам отсутствует (nil), если у видео нет глав, а к репозиторию глав для
// удалённого видео (VideoID == nil) не обращаются вовсе.
func TestService_AssignmentGet_ChapterProgressAbsentWithoutChaptersOrVideo(t *testing.T) {
	t.Parallel()

	t.Run("видео без глав", func(t *testing.T) {
		t.Parallel()

		f := newAssignmentFixture()
		userID := uuid.New()
		mc := minimock.NewController(t)
		m := newAssignmentMocks(mc)

		m.Assignment.SelectByIDMock.Return(domain.Assignment{
			ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID,
			CreatedBy: f.InitiatorID,
		}, nil)
		m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
		m.Participants.SelectByAssignmentIDsMock.Return([]domain.AssignmentParticipant{
			{AssignmentID: f.AssignmentID, UserID: userID, Status: domain.AssignmentParticipantStatusAssigned},
		}, nil)
		m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
		m.Events.SelectByAssignmentIDMock.Return(nil, nil)
		m.User.GetByIDsMock.Return([]domain.User{{ID: userID, Name: "Иван"}}, nil)
		m.Video.SelectMock.Return(&f.Video, nil)
		m.Progress.SelectByVideoIDsMock.Return(nil, nil)
		m.Access.CanWatchVideoMock.Return(true)
		m.Chapters.SelectBoundsByVideoIDMock.Return(nil, nil)

		svc := newAssignmentService(m, f.Cfg, f.Now)
		details, err := svc.Get(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

		require.NoError(t, err)
		require.Len(t, details.Participants, 1)
		require.Nil(t, details.Participants[0].ChapterProgress)
		require.Zero(t, m.Chapters.SelectProgressByVideoAndUsersAfterCounter())
	})

	t.Run("видео удалено", func(t *testing.T) {
		t.Parallel()

		f := newAssignmentFixture()
		userID := uuid.New()
		mc := minimock.NewController(t)
		m := newAssignmentMocks(mc)

		m.Assignment.SelectByIDMock.Return(domain.Assignment{
			ID: f.AssignmentID, AccountID: f.AccountID, GroupID: &f.GroupID, CreatedBy: f.InitiatorID,
		}, nil)
		m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
		m.Participants.SelectByAssignmentIDsMock.Return([]domain.AssignmentParticipant{
			{AssignmentID: f.AssignmentID, UserID: userID, Status: domain.AssignmentParticipantStatusCompleted},
		}, nil)
		m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
		m.Events.SelectByAssignmentIDMock.Return(nil, nil)
		m.User.GetByIDsMock.Return([]domain.User{{ID: userID, Name: "Иван"}}, nil)

		svc := newAssignmentService(m, f.Cfg, f.Now)
		details, err := svc.Get(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

		require.NoError(t, err)
		require.Len(t, details.Participants, 1)
		require.Nil(t, details.Participants[0].ChapterProgress)
		require.Zero(t, m.Chapters.SelectBoundsByVideoIDAfterCounter())
	})
}

// TestService_AssignmentGet_ChapterProgressFollowsCurrentBoundsNotCompletion проверяет решение
// В-6 файла эпика Э4 (КП-5): правка границ глав меняет детализацию по текущей разметке, но
// зачёт видео (Status/CompletedAt участника) от неё никак не зависит.
func TestService_AssignmentGet_ChapterProgressFollowsCurrentBoundsNotCompletion(t *testing.T) {
	t.Parallel()

	completedAt := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	coveragePct := 100

	buildDetails := func(t *testing.T, bounds []domain.ChapterBound) domain.AssignmentDetails {
		t.Helper()

		f := newAssignmentFixture()
		userID := uuid.New()
		mc := minimock.NewController(t)
		m := newAssignmentMocks(mc)

		participant := domain.AssignmentParticipant{
			AssignmentID: f.AssignmentID, UserID: userID,
			Status:      domain.AssignmentParticipantStatusCompleted,
			CompletedAt: &completedAt, CompletedCoveragePct: &coveragePct,
		}

		m.Assignment.SelectByIDMock.Return(domain.Assignment{
			ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, GroupID: &f.GroupID,
			CreatedBy: f.InitiatorID,
		}, nil)
		m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
		m.Participants.SelectByAssignmentIDsMock.Return([]domain.AssignmentParticipant{participant}, nil)
		m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
		m.Events.SelectByAssignmentIDMock.Return(nil, nil)
		m.User.GetByIDsMock.Return([]domain.User{{ID: userID, Name: "Иван"}}, nil)
		m.Video.SelectMock.Return(&f.Video, nil)
		m.Progress.SelectByVideoIDsMock.Return(nil, nil)
		m.Chapters.SelectBoundsByVideoIDMock.Return(bounds, nil)
		m.Chapters.SelectProgressByVideoAndUsersMock.Return(nil, nil)

		svc := newAssignmentService(m, f.Cfg, f.Now)
		details, err := svc.Get(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)
		require.NoError(t, err)

		return details
	}

	before := buildDetails(t, []domain.ChapterBound{
		{Chapter: domain.Chapter{ID: uuid.New(), Name: "Один блок", StartMs: 0}, EndMs: 10000},
	})
	after := buildDetails(t, []domain.ChapterBound{
		{Chapter: domain.Chapter{ID: uuid.New(), Name: "Часть 1", StartMs: 0}, EndMs: 5000},
		{Chapter: domain.Chapter{ID: uuid.New(), Name: "Часть 2", StartMs: 5000}, EndMs: 10000},
	})

	// Зачёт видео не зависит от разметки глав (BR-47, В-6).
	require.Equal(t, domain.AssignmentParticipantStatusCompleted, before.Participants[0].Participant.Status)
	require.Equal(t, domain.AssignmentParticipantStatusCompleted, after.Participants[0].Participant.Status)
	require.Equal(t, before.Participants[0].Participant.CompletedAt, after.Participants[0].Participant.CompletedAt)

	// Детализация по главам считается заново по текущей разметке.
	require.Equal(t, 1, before.Participants[0].ChapterProgress.Total)
	require.Equal(t, 2, after.Participants[0].ChapterProgress.Total)
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

// TestService_AssignmentList_EmptyWhenNoAssignments проверяет, что список пуст, если область
// В-8 инициатора не содержит ни одного назначения — без дальнейших батч-запросов.
func TestService_AssignmentList_EmptyWhenNoAssignments(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Assignment.SelectByFilterMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.List(t.Context(), f.AccountID, f.InitiatorID, domain.AssignmentFilter{})

	require.NoError(t, err)
	require.Empty(t, items)
}

// TestService_AssignmentList_PassesScopeAndFiltersToRepository проверяет перевод области В-8
// (не-владелец, ограниченный список групп) и всех query-фильтров в repository.AssignmentFilter
// (§4, §5 дизайна эпика Э3, В-53).
func TestService_AssignmentList_PassesScopeAndFiltersToRepository(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	groups := []uuid.UUID{f.GroupID}
	statusActive := domain.AssignmentStatusActive
	dueFrom := f.Now
	dueTo := f.Now.Add(30 * 24 * time.Hour)
	userID := uuid.New()

	m.Access.ManagedAssignmentGroupsMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID).
		Return(false, groups, nil)
	m.Assignment.SelectByFilterMock.Set(
		func(_ context.Context, got repository.AssignmentFilter) ([]domain.Assignment, error) {
			require.Equal(t, f.AccountID, got.AccountID)
			require.False(t, got.Scope.All)
			require.Equal(t, groups, got.Scope.GroupIDs)
			require.Equal(t, f.InitiatorID, got.Scope.CreatedBy)
			require.Equal(t, &f.GroupID, got.GroupID)
			require.Equal(t, &userID, got.UserID)
			require.Equal(t, &statusActive, got.Status)
			require.Equal(t, &dueFrom, got.DueFrom)
			require.Equal(t, &dueTo, got.DueTo)

			return nil, nil
		},
	)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, err := svc.List(t.Context(), f.AccountID, f.InitiatorID, domain.AssignmentFilter{
		GroupID: &f.GroupID, UserID: &userID, Status: &statusActive, DueFrom: &dueFrom, DueTo: &dueTo,
	})

	require.NoError(t, err)
}

// TestService_AssignmentList_BuildsTargetsCountersAndCreatedBy проверяет сборку целей (имя
// пользователя/снимок группы), счётчиков и автора назначения одним проходом батчей без участия
// (ExpandParticipants=false).
func TestService_AssignmentList_BuildsTargetsCountersAndCreatedBy(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := activeAssignment(f)
	userTargetID := uuid.New()

	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Assignment.SelectByFilterMock.Return([]domain.Assignment{assignment}, nil)
	m.Targets.SelectByAssignmentIDsMock.Expect(minimock.AnyContext, []uuid.UUID{f.AssignmentID}).
		Return([]domain.AssignmentTarget{
			{AssignmentID: f.AssignmentID, TargetType: domain.AssignmentTargetTypeUser, TargetID: userTargetID},
			{AssignmentID: f.AssignmentID, TargetType: domain.AssignmentTargetTypeGroup, TargetID: f.GroupID},
		}, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: f.InitiatorID, Name: "Автор"},
		{ID: userTargetID, Name: "Иван", Surname: "Иванов"},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.
		Expect(minimock.AnyContext, []uuid.UUID{f.AssignmentID}, false).
		Return(map[uuid.UUID]domain.AssignmentCounters{f.AssignmentID: {Total: 3, Assigned: 3}}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.List(t.Context(), f.AccountID, f.InitiatorID, domain.AssignmentFilter{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Автор", items[0].CreatedByUser.Name)
	require.Len(t, items[0].Targets, 2)
	require.Equal(t, 3, items[0].Counters.Total)
	require.Nil(t, items[0].Participants, "участники не запрошены — expand_participants=false")
}

// TestService_AssignmentList_ExpandParticipantsExcludesDeactivatedByDefault проверяет
// критерий В-53/Э3-Т29: include_deactivated=false по умолчанию скрывает деактивированного
// участника из счётчиков (переданный в CountByAssignmentIDs флаг) и из списка участников.
func TestService_AssignmentList_ExpandParticipantsExcludesDeactivatedByDefault(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := activeAssignment(f)
	activeUserID := uuid.New()
	deactivatedUserID := uuid.New()
	deactivatedAt := f.Now.Add(-time.Hour)

	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Assignment.SelectByFilterMock.Return([]domain.Assignment{assignment}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Expect(minimock.AnyContext, []uuid.UUID{f.AssignmentID}).
		Return([]domain.AssignmentParticipant{
			{
				AssignmentID: f.AssignmentID, UserID: activeUserID,
				Status: domain.AssignmentParticipantStatusInProgress, DueAt: f.Now.Add(time.Hour),
			},
			{
				AssignmentID: f.AssignmentID, UserID: deactivatedUserID,
				Status: domain.AssignmentParticipantStatusAssigned, DueAt: f.Now.Add(time.Hour),
			},
		}, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: f.InitiatorID, Name: "Автор"},
		{ID: activeUserID, Name: "Активный"},
		{ID: deactivatedUserID, Name: "Уволен", DeactivatedAt: &deactivatedAt},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.
		Expect(minimock.AnyContext, []uuid.UUID{f.AssignmentID}, false).
		Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Video.SelectByIDsMock.Return([]domain.Video{f.Video}, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	m.Access.CanWatchVideoMock.Return(true)
	// Видео без глав — сводка по главам не запрашивается (Э4-Т4).
	m.Chapters.SelectBoundsByVideoIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.List(t.Context(), f.AccountID, f.InitiatorID, domain.AssignmentFilter{ExpandParticipants: true})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Participants, 1, "деактивированный участник исключён из списка")
	require.Equal(t, activeUserID, items[0].Participants[0].Participant.UserID)
	require.True(t, items[0].Participants[0].HasAccess)
}

// TestService_AssignmentList_IncludeDeactivatedKeepsAllParticipants проверяет переключатель
// include_deactivated=true: деактивированный участник остаётся в списке, флаг доходит до
// CountByAssignmentIDs.
func TestService_AssignmentList_IncludeDeactivatedKeepsAllParticipants(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := activeAssignment(f)
	activeUserID := uuid.New()
	deactivatedUserID := uuid.New()
	deactivatedAt := f.Now.Add(-time.Hour)

	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Assignment.SelectByFilterMock.Return([]domain.Assignment{assignment}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return([]domain.AssignmentParticipant{
		{
			AssignmentID: f.AssignmentID, UserID: activeUserID,
			Status: domain.AssignmentParticipantStatusInProgress, DueAt: f.Now.Add(time.Hour),
		},
		{
			AssignmentID: f.AssignmentID, UserID: deactivatedUserID,
			Status: domain.AssignmentParticipantStatusAssigned, DueAt: f.Now.Add(time.Hour),
		},
	}, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: f.InitiatorID, Name: "Автор"},
		{ID: activeUserID, Name: "Активный"},
		{ID: deactivatedUserID, Name: "Уволен", DeactivatedAt: &deactivatedAt},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.
		Expect(minimock.AnyContext, []uuid.UUID{f.AssignmentID}, true).
		Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Video.SelectByIDsMock.Return([]domain.Video{f.Video}, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	m.Access.CanWatchVideoMock.Return(true)
	// Видео без глав — сводка по главам не запрашивается (Э4-Т4).
	m.Chapters.SelectBoundsByVideoIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.List(t.Context(), f.AccountID, f.InitiatorID, domain.AssignmentFilter{
		ExpandParticipants: true, IncludeDeactivated: true,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Participants, 2)
}

// TestService_AssignmentList_ExpandParticipantsIncludesChapterTotalsOnly проверяет §6 дизайна
// эпика Э4: сводный список назначений (expand_participants=true) отдаёт по главам только
// Total/Completed — раскрываемая детализация (Chapters) туда не попадает, чтобы не раздувать
// ответ на сотне назначений.
func TestService_AssignmentList_ExpandParticipantsIncludesChapterTotalsOnly(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	f.Video.DurationMs = ptrInt64(10000)
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := activeAssignment(f)
	userID := uuid.New()

	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Assignment.SelectByFilterMock.Return([]domain.Assignment{assignment}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return([]domain.AssignmentParticipant{
		{
			AssignmentID: f.AssignmentID, UserID: userID,
			Status: domain.AssignmentParticipantStatusInProgress, DueAt: f.Now.Add(time.Hour),
		},
	}, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: f.InitiatorID, Name: "Автор"}, {ID: userID, Name: "Сотрудник"},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Video.SelectByIDsMock.Return([]domain.Video{f.Video}, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	m.Access.CanWatchVideoMock.Return(true)

	bounds := []domain.ChapterBound{
		{Chapter: domain.Chapter{ID: uuid.New(), Name: "Глава 1", StartMs: 0}, EndMs: 10000},
	}
	m.Chapters.SelectBoundsByVideoIDMock.Return(bounds, nil)
	m.Chapters.SelectProgressByVideoAndUsersMock.Return([]domain.ChapterUserProgress{
		{ChapterProgress: domain.ChapterProgress{ChapterBound: bounds[0], CoveredMs: 10000}, UserID: userID},
	}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.List(t.Context(), f.AccountID, f.InitiatorID, domain.AssignmentFilter{ExpandParticipants: true})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Participants, 1)

	summary := items[0].Participants[0].ChapterProgress
	require.NotNil(t, summary)
	require.Equal(t, 1, summary.Total)
	require.Equal(t, 1, summary.Completed)
	require.Nil(t, summary.Chapters, "сводный список не раскрывает детализацию по главам (§6)")
}

// TestService_AssignmentListForUser_NotFoundWhenUserNotInAccount проверяет 404 для сотрудника
// вне аккаунта — до любого обращения к назначениям.
func TestService_AssignmentListForUser_NotFoundWhenUserNotInAccount(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	targetUserID := uuid.New()
	roleID := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.User.GetByIDMock.Expect(minimock.AnyContext, targetUserID).
		Return([]domain.User{{ID: targetUserID, RoleID: roleID}}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{{ID: roleID, AccountID: uuid.New()}}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	_, err := svc.ListForUser(t.Context(), f.AccountID, f.InitiatorID, targetUserID)

	require.ErrorIs(t, err, service.ErrNotFound)
}

// TestService_AssignmentListForUser_EmptyWhenNoParticipants проверяет пустой отчёт для
// сотрудника без единого назначения.
func TestService_AssignmentListForUser_EmptyWhenNoParticipants(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	targetUserID := uuid.New()
	roleID := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.User.GetByIDMock.Expect(minimock.AnyContext, targetUserID).
		Return([]domain.User{{ID: targetUserID, RoleID: roleID}}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{{ID: roleID, AccountID: f.AccountID}}, nil)
	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Participants.SelectByUserIDMock.Expect(minimock.AnyContext, targetUserID).Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.ListForUser(t.Context(), f.AccountID, f.InitiatorID, targetUserID)

	require.NoError(t, err)
	require.Empty(t, items)
}

// TestService_AssignmentListForUser_FiltersByScope проверяет критерий КП-6: руководитель
// группы не видит в отчёте по сотруднику назначения из чужой группы, созданные не им.
func TestService_AssignmentListForUser_FiltersByScope(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	targetUserID := uuid.New()
	roleID := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	inScope := activeAssignment(f)
	outOfScopeGroupID := uuid.New()
	outOfScope := domain.Assignment{
		ID: uuid.New(), AccountID: f.AccountID, GroupID: &outOfScopeGroupID, CreatedBy: uuid.New(),
	}

	participantIn := domain.AssignmentParticipant{
		AssignmentID: inScope.ID, UserID: targetUserID, DueAt: f.Now.Add(time.Hour),
	}
	participantOut := domain.AssignmentParticipant{
		AssignmentID: outOfScope.ID, UserID: targetUserID, DueAt: f.Now.Add(time.Hour),
	}

	m.User.GetByIDMock.Expect(minimock.AnyContext, targetUserID).
		Return([]domain.User{{ID: targetUserID, RoleID: roleID}}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{{ID: roleID, AccountID: f.AccountID}}, nil)
	m.Access.ManagedAssignmentGroupsMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID).
		Return(false, []uuid.UUID{f.GroupID}, nil)
	m.Participants.SelectByUserIDMock.Expect(minimock.AnyContext, targetUserID).
		Return([]domain.AssignmentParticipant{participantIn, participantOut}, nil)
	m.Assignment.SelectByIDsMock.Return([]domain.Assignment{inScope, outOfScope}, nil)
	m.Targets.SelectByAssignmentIDsMock.Expect(minimock.AnyContext, []uuid.UUID{inScope.ID}).
		Return(nil, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: f.InitiatorID, Name: "Автор"}, {ID: targetUserID, Name: "Сотрудник"},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.
		Expect(minimock.AnyContext, []uuid.UUID{inScope.ID}, true).
		Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	// Прогресс/длительность батчатся только по назначению в области — вне области видео не
	// запрашивается (см. правку ListForUser, сужение assignmentByID).
	m.Video.SelectByIDsMock.Expect(minimock.AnyContext, []uuid.UUID{f.VideoID}).
		Return([]domain.Video{f.Video}, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)
	// Видео без глав — сводка по главам не запрашивается (Э4-Т4).
	m.Chapters.SelectBoundsByVideoIDMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.ListForUser(t.Context(), f.AccountID, f.InitiatorID, targetUserID)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, inScope.ID, items[0].Assignment.ID)
}

// TestService_AssignmentListForUser_IncludesFullChapterDetail проверяет §6 дизайна эпика Э4:
// отчёт по сотруднику, в отличие от сводного списка назначений, отдаёт полную раскрываемую
// детализацию по главам на каждое назначение с главами.
func TestService_AssignmentListForUser_IncludesFullChapterDetail(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	f.Video.DurationMs = ptrInt64(10000)
	targetUserID := uuid.New()
	roleID := uuid.New()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := activeAssignment(f)
	participant := domain.AssignmentParticipant{
		AssignmentID: assignment.ID, UserID: targetUserID, DueAt: f.Now.Add(time.Hour),
	}

	m.User.GetByIDMock.Expect(minimock.AnyContext, targetUserID).
		Return([]domain.User{{ID: targetUserID, RoleID: roleID}}, nil)
	m.AccountRole.GetByIDMock.Return([]domain.AccountRole{{ID: roleID, AccountID: f.AccountID}}, nil)
	m.Access.ManagedAssignmentGroupsMock.Return(true, nil, nil)
	m.Participants.SelectByUserIDMock.Expect(minimock.AnyContext, targetUserID).
		Return([]domain.AssignmentParticipant{participant}, nil)
	m.Assignment.SelectByIDsMock.Return([]domain.Assignment{assignment}, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.User.GetByIDsMock.Return([]domain.User{
		{ID: f.InitiatorID, Name: "Автор"}, {ID: targetUserID, Name: "Сотрудник"},
	}, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Video.SelectByIDsMock.Return([]domain.Video{f.Video}, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)

	bounds := []domain.ChapterBound{
		{Chapter: domain.Chapter{ID: uuid.New(), Name: "Глава 1", StartMs: 0}, EndMs: 10000},
	}
	m.Chapters.SelectBoundsByVideoIDMock.Return(bounds, nil)
	m.Chapters.SelectProgressByVideoAndUsersMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	items, err := svc.ListForUser(t.Context(), f.AccountID, f.InitiatorID, targetUserID)

	require.NoError(t, err)
	require.Len(t, items, 1)

	summary := items[0].Participant.ChapterProgress
	require.NotNil(t, summary)
	require.Equal(t, 1, summary.Total)
	require.Equal(t, []domain.ParticipantChapterStatus{
		{Name: "Глава 1", CoveragePct: 0, Status: domain.ChapterStatusNotStarted},
	}, summary.Chapters)
}

func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt64(v int64) *int64        { return &v }

// activeAssignment — действующее назначение фикстуры в режиме "дата".
func activeAssignment(f assignmentFixture) domain.Assignment {
	dueAt := f.Now.Add(7 * 24 * time.Hour)

	return domain.Assignment{
		ID: f.AssignmentID, AccountID: f.AccountID, VideoID: &f.VideoID, VideoName: f.Video.Name,
		GroupID: &f.GroupID, GroupName: "Продажи", CreatedBy: f.InitiatorID,
		DueMode: domain.AssignmentDueModeDate, DueAt: &dueAt, Status: domain.AssignmentStatusActive,
	}
}

// expectAssignmentCardCalls настраивает моки хвоста Get — сборки карточки назначения после
// изменения (цели, участники, счётчики, журнал, автор).
func expectAssignmentCardCalls(m assignmentMocks, assignment domain.Assignment) {
	m.Assignment.SelectByIDMock.Return(assignment, nil)
	m.Targets.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.SelectByAssignmentIDsMock.Return(nil, nil)
	m.Participants.CountByAssignmentIDsMock.Return(map[uuid.UUID]domain.AssignmentCounters{}, nil)
	m.Events.SelectByAssignmentIDMock.Return(nil, nil)
	m.User.GetByIDsMock.Return(nil, nil)
}

// TestService_AssignmentUpdateDue_RecalculatesParticipantsAndLogsEvent проверяет изменение
// срока (Э3-Т20/КП-9): назначение и персональные сроки незавершённых участников обновляются,
// в журнал пишется due_changed.
func TestService_AssignmentUpdateDue_RecalculatesParticipantsAndLogsEvent(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	assignment := activeAssignment(f)
	m.Access.CanManageAssignmentsMock.Return(nil)
	expectAssignmentCardCalls(m, assignment)

	dueDays := 10
	var updatedMode domain.AssignmentDueMode
	var updatedDays *int
	m.Assignment.UpdateDueMock.Set(func(
		_ context.Context, _ uuid.UUID, mode domain.AssignmentDueMode, _ *time.Time, days *int,
	) (domain.Assignment, error) {
		updatedMode, updatedDays = mode, days
		return assignment, nil
	})

	var participantsMode domain.AssignmentDueMode
	m.Participants.UpdateDueByAssignmentMock.Set(func(
		_ context.Context, _ uuid.UUID, mode domain.AssignmentDueMode, _ *time.Time, _ *int,
	) ([]uuid.UUID, error) {
		participantsMode = mode
		return []uuid.UUID{uuid.New()}, nil
	})

	var loggedType domain.AssignmentEventType
	m.Events.InsertMock.Set(func(
		_ context.Context, _ uuid.UUID, _ *uuid.UUID,
		eventType domain.AssignmentEventType, _ *uuid.UUID, _ json.RawMessage, _ time.Time,
	) (domain.AssignmentEvent, error) {
		loggedType = eventType
		return domain.AssignmentEvent{}, nil
	})

	svc := newAssignmentService(m, f.Cfg, f.Now)
	mode := domain.AssignmentDueModeDays

	_, err := svc.UpdateDue(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, domain.UpdateAssignment{
		DueMode: &mode, DueDays: &dueDays,
	})

	require.NoError(t, err)
	require.Equal(t, domain.AssignmentDueModeDays, updatedMode)
	require.NotNil(t, updatedDays)
	require.Equal(t, dueDays, *updatedDays)
	require.Equal(t, domain.AssignmentDueModeDays, participantsMode)
	require.Equal(t, domain.AssignmentEventTypeDueChanged, loggedType)
}

// TestService_AssignmentUpdateDue_ValidatesNewDue проверяет, что новый срок проходит те же
// проверки, что и при создании (В-6).
func TestService_AssignmentUpdateDue_ValidatesNewDue(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Return(activeAssignment(f), nil)
	m.Access.CanManageAssignmentsMock.Return(nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	mode := domain.AssignmentDueModeDate
	past := f.Now.Add(-time.Hour)

	_, err := svc.UpdateDue(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, domain.UpdateAssignment{
		DueMode: &mode, DueAt: &past,
	})

	require.ErrorIs(t, err, service.ErrDueAtInvalid)
}

// TestService_AssignmentUpdateDue_CancelledIsConflict — отменённое назначение не редактируется.
func TestService_AssignmentUpdateDue_CancelledIsConflict(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	cancelled := activeAssignment(f)
	cancelled.Status = domain.AssignmentStatusCancelled
	m.Assignment.SelectByIDMock.Return(cancelled, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	comment := "новый комментарий"

	_, err := svc.UpdateDue(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, domain.UpdateAssignment{
		Comment: &comment,
	})

	require.ErrorIs(t, err, service.ErrAssignmentCancelled)
}

// TestService_AssignmentUpdateDue_ForeignAccountIsNotFound — назначение чужого аккаунта
// невидимо (изоляция арендатора).
func TestService_AssignmentUpdateDue_ForeignAccountIsNotFound(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	foreign := activeAssignment(f)
	foreign.AccountID = uuid.New()
	m.Assignment.SelectByIDMock.Return(foreign, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)
	comment := ""

	_, err := svc.UpdateDue(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, domain.UpdateAssignment{
		Comment: &comment,
	})

	require.ErrorIs(t, err, service.ErrNotFound)
}

// TestService_AssignmentCancel_CancelsParticipantsAndLogsEvents проверяет отмену назначения
// (Э3-Т21): незавершённые участники переходят в cancelled, на каждого пишется событие.
func TestService_AssignmentCancel_CancelsParticipantsAndLogsEvents(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Return(activeAssignment(f), nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.Assignment.CancelMock.Return(true, nil)

	cancelledUsers := []uuid.UUID{uuid.New(), uuid.New()}
	var participantReason domain.AssignmentParticipantCancelReason
	m.Participants.CancelByAssignmentMock.Set(func(
		_ context.Context, _ uuid.UUID, reason domain.AssignmentParticipantCancelReason, _ time.Time,
	) ([]uuid.UUID, error) {
		participantReason = reason
		return cancelledUsers, nil
	})

	var loggedEvents []domain.AssignmentEvent
	m.Events.InsertBatchMock.Set(func(
		_ context.Context, e []domain.AssignmentEvent,
	) ([]domain.AssignmentEvent, error) {
		loggedEvents = e
		return e, nil
	})

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.Cancel(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

	require.NoError(t, err)
	require.Equal(t, domain.AssignmentParticipantCancelReasonAssignmentCancelled, participantReason)
	require.Len(t, loggedEvents, len(cancelledUsers))
	for _, event := range loggedEvents {
		require.Equal(t, domain.AssignmentEventTypeParticipantCancelled, event.Type)
	}
}

// TestService_AssignmentCancel_RepeatIsConflict — повторная отмена назначения возвращает 409.
func TestService_AssignmentCancel_RepeatIsConflict(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	cancelled := activeAssignment(f)
	cancelled.Status = domain.AssignmentStatusCancelled
	m.Assignment.SelectByIDMock.Return(cancelled, nil)
	m.Access.CanManageAssignmentsMock.Return(nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.Cancel(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID)

	require.ErrorIs(t, err, service.ErrAssignmentCancelled)
}

// TestService_AssignmentRemoveParticipant_CompletedIsConflict — завершившего обучение
// участника снять нельзя (Э3-Т22, Э3-Н1).
func TestService_AssignmentRemoveParticipant_CompletedIsConflict(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	userID := uuid.New()
	m.Assignment.SelectByIDMock.Return(activeAssignment(f), nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.Participants.SelectByAssignmentIDAndUserIDMock.Return(domain.AssignmentParticipant{
		AssignmentID: f.AssignmentID, UserID: userID, Status: domain.AssignmentParticipantStatusCompleted,
	}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.RemoveParticipant(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, userID)

	require.ErrorIs(t, err, service.ErrParticipantCompleted)
}

// TestService_AssignmentRemoveParticipant_MissingIsNotFound — участника нет в назначении.
func TestService_AssignmentRemoveParticipant_MissingIsNotFound(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectByIDMock.Return(activeAssignment(f), nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.Participants.SelectByAssignmentIDAndUserIDMock.Return(
		domain.AssignmentParticipant{}, repository.ErrNotFound,
	)

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.RemoveParticipant(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, uuid.New())

	require.ErrorIs(t, err, service.ErrNotFound)
}

// TestService_AssignmentRemoveParticipant_CancelsAndLogs — снятие активного участника.
func TestService_AssignmentRemoveParticipant_CancelsAndLogs(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	userID := uuid.New()
	m.Assignment.SelectByIDMock.Return(activeAssignment(f), nil)
	m.Access.CanManageAssignmentsMock.Return(nil)
	m.Participants.SelectByAssignmentIDAndUserIDMock.Return(domain.AssignmentParticipant{
		AssignmentID: f.AssignmentID, UserID: userID, Status: domain.AssignmentParticipantStatusInProgress,
	}, nil)

	var cancelReason domain.AssignmentParticipantCancelReason
	m.Participants.CancelOneMock.Set(func(
		_ context.Context, _, _ uuid.UUID, reason domain.AssignmentParticipantCancelReason, _ time.Time,
	) (bool, error) {
		cancelReason = reason
		return true, nil
	})

	var loggedEvents []domain.AssignmentEvent
	m.Events.InsertBatchMock.Set(func(
		_ context.Context, e []domain.AssignmentEvent,
	) ([]domain.AssignmentEvent, error) {
		loggedEvents = e
		return e, nil
	})

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.RemoveParticipant(t.Context(), f.AccountID, f.InitiatorID, f.AssignmentID, userID)

	require.NoError(t, err)
	require.Equal(t, domain.AssignmentParticipantCancelReasonRemovedByManager, cancelReason)
	require.Len(t, loggedEvents, 1)
	require.Equal(t, domain.AssignmentEventTypeParticipantCancelled, loggedEvents[0].Type)
	require.NotNil(t, loggedEvents[0].UserID)
	require.Equal(t, userID, *loggedEvents[0].UserID)
}

// TestService_AssignmentOnMembersAdded_EnrollsNewcomers проверяет каскад добавления в группу
// (Э3-Т3/КП-7): новичок получает персональную запись со сроком по режиму «N дней» от момента
// зачисления.
func TestService_AssignmentOnMembersAdded_EnrollsNewcomers(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	dueDays := 3
	assignment := activeAssignment(f)
	assignment.DueMode = domain.AssignmentDueModeDays
	assignment.DueAt = nil
	assignment.DueDays = &dueDays

	m.Assignment.SelectActiveByTargetGroupMock.Return([]domain.Assignment{assignment}, nil)
	m.Video.SelectMock.Return(&f.Video, nil)
	m.Progress.SelectByVideoIDsMock.Return(nil, nil)

	var enrolled []domain.AssignmentParticipant
	m.Participants.InsertBatchMock.Set(func(
		_ context.Context, p []domain.AssignmentParticipant,
	) ([]domain.AssignmentParticipant, error) {
		enrolled = p
		return p, nil
	})

	var loggedEvents []domain.AssignmentEvent
	m.Events.InsertBatchMock.Set(func(
		_ context.Context, e []domain.AssignmentEvent,
	) ([]domain.AssignmentEvent, error) {
		loggedEvents = e
		return e, nil
	})

	newcomerID := uuid.New()
	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.OnMembersAdded(t.Context(), f.GroupID, []uuid.UUID{newcomerID})

	require.NoError(t, err)
	require.Len(t, enrolled, 1)
	require.Equal(t, newcomerID, enrolled[0].UserID)
	require.Equal(t, domain.AssignmentParticipantSourceGroup, enrolled[0].Source)
	require.NotNil(t, enrolled[0].SourceGroupID)
	require.Equal(t, f.GroupID, *enrolled[0].SourceGroupID)
	require.Equal(t, f.Now.AddDate(0, 0, dueDays), enrolled[0].DueAt)
	require.Equal(t, domain.AssignmentParticipantStatusAssigned, enrolled[0].Status)
	require.Len(t, loggedEvents, 1)
	require.Equal(t, domain.AssignmentEventTypeParticipantEnrolled, loggedEvents[0].Type)
}

// TestService_AssignmentOnMembersAdded_SkipsExpiredDateAssignment — правило В-5: назначение с
// прошедшей фиксированной датой новичкам не выдаётся.
func TestService_AssignmentOnMembersAdded_SkipsExpiredDateAssignment(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	expired := activeAssignment(f)
	expired.DueAt = ptrTime(f.Now.Add(-time.Hour))
	m.Assignment.SelectActiveByTargetGroupMock.Return([]domain.Assignment{expired}, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.OnMembersAdded(t.Context(), f.GroupID, []uuid.UUID{uuid.New()})

	require.NoError(t, err)
}

// TestService_AssignmentOnMembersAdded_CompletesAlreadyWatched — новичок, досмотревший видео
// раньше, зачисляется сразу выполненным (В-11).
func TestService_AssignmentOnMembersAdded_CompletesAlreadyWatched(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	newcomerID := uuid.New()
	video := f.Video
	video.DurationMs = ptrInt64(100_000)

	m.Assignment.SelectActiveByTargetGroupMock.Return([]domain.Assignment{activeAssignment(f)}, nil)
	m.Video.SelectMock.Return(&video, nil)
	m.Progress.SelectByVideoIDsMock.Return([]domain.WatchProgress{
		{
			UserID: newcomerID, VideoID: f.VideoID, CoveredMs: 100_000,
			ThresholdReachedAt: ptrTime(f.Now.Add(-24 * time.Hour)),
		},
	}, nil)

	var enrolled []domain.AssignmentParticipant
	m.Participants.InsertBatchMock.Set(func(
		_ context.Context, p []domain.AssignmentParticipant,
	) ([]domain.AssignmentParticipant, error) {
		enrolled = p
		return p, nil
	})

	var loggedEvents []domain.AssignmentEvent
	m.Events.InsertBatchMock.Set(func(
		_ context.Context, e []domain.AssignmentEvent,
	) ([]domain.AssignmentEvent, error) {
		loggedEvents = e
		return e, nil
	})

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.OnMembersAdded(t.Context(), f.GroupID, []uuid.UUID{newcomerID})

	require.NoError(t, err)
	require.Len(t, enrolled, 1)
	require.Equal(t, domain.AssignmentParticipantStatusCompleted, enrolled[0].Status)
	require.Len(t, loggedEvents, 2, "зачисление и подтверждение просмотра")
	require.Equal(t, domain.AssignmentEventTypeParticipantCompleted, loggedEvents[1].Type)
}

// TestService_AssignmentOnMemberRemoved_CancelsGroupParticipations проверяет каскад исключения
// из группы (Э3-Т30): участия через группу отменяются с причиной left_group.
func TestService_AssignmentOnMemberRemoved_CancelsGroupParticipations(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	userID := uuid.New()
	var reason domain.AssignmentParticipantCancelReason
	m.Participants.CancelBySourceGroupAndUserMock.Set(func(
		_ context.Context, _, _ uuid.UUID, r domain.AssignmentParticipantCancelReason, _ time.Time,
	) ([]uuid.UUID, error) {
		reason = r
		return []uuid.UUID{f.AssignmentID}, nil
	})

	var loggedEvents []domain.AssignmentEvent
	m.Events.InsertBatchMock.Set(func(
		_ context.Context, e []domain.AssignmentEvent,
	) ([]domain.AssignmentEvent, error) {
		loggedEvents = e
		return e, nil
	})

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.OnMemberRemoved(t.Context(), f.GroupID, userID)

	require.NoError(t, err)
	require.Equal(t, domain.AssignmentParticipantCancelReasonLeftGroup, reason)
	require.Len(t, loggedEvents, 1)
	require.Equal(t, domain.AssignmentEventTypeParticipantCancelled, loggedEvents[0].Type)
}

// TestService_AssignmentOnVideoDeleted_CancelsAssignments проверяет каскад удаления видео
// (Э3-Т28): назначение и незавершённые участия отменяются с причиной video_deleted.
func TestService_AssignmentOnVideoDeleted_CancelsAssignments(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectActiveByVideoIDsMock.Return([]domain.Assignment{activeAssignment(f)}, nil)

	var assignmentReason domain.AssignmentCancelReason
	var actor *uuid.UUID
	m.Assignment.CancelMock.Set(func(
		_ context.Context, _ uuid.UUID, cancelledBy *uuid.UUID,
		reason domain.AssignmentCancelReason, _ time.Time,
	) (bool, error) {
		assignmentReason, actor = reason, cancelledBy
		return true, nil
	})

	m.Events.InsertMock.Return(domain.AssignmentEvent{}, nil)

	var participantReason domain.AssignmentParticipantCancelReason
	m.Participants.CancelByAssignmentMock.Set(func(
		_ context.Context, _ uuid.UUID, reason domain.AssignmentParticipantCancelReason, _ time.Time,
	) ([]uuid.UUID, error) {
		participantReason = reason
		return []uuid.UUID{uuid.New()}, nil
	})
	m.Events.InsertBatchMock.Return(nil, nil)

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.OnVideoDeleted(t.Context(), f.VideoID)

	require.NoError(t, err)
	require.Equal(t, domain.AssignmentCancelReasonVideoDeleted, assignmentReason)
	require.Nil(t, actor, "системная отмена выполняется без инициатора")
	require.Equal(t, domain.AssignmentParticipantCancelReasonVideoDeleted, participantReason)
}

// TestService_AssignmentOnGroupDeleted_CancelsAssignments проверяет каскад удаления группы
// (Э3-Т31): назначения видео группы отменяются с причиной group_deleted.
func TestService_AssignmentOnGroupDeleted_CancelsAssignments(t *testing.T) {
	t.Parallel()

	f := newAssignmentFixture()
	mc := minimock.NewController(t)
	m := newAssignmentMocks(mc)

	m.Assignment.SelectActiveByGroupIDMock.Return([]domain.Assignment{activeAssignment(f)}, nil)
	m.Assignment.CancelMock.Return(true, nil)
	m.Events.InsertMock.Return(domain.AssignmentEvent{}, nil)

	var participantReason domain.AssignmentParticipantCancelReason
	m.Participants.CancelByAssignmentMock.Set(func(
		_ context.Context, _ uuid.UUID, reason domain.AssignmentParticipantCancelReason, _ time.Time,
	) ([]uuid.UUID, error) {
		participantReason = reason
		return nil, nil
	})

	svc := newAssignmentService(m, f.Cfg, f.Now)

	err := svc.OnGroupDeleted(t.Context(), f.GroupID)

	require.NoError(t, err)
	require.Equal(t, domain.AssignmentParticipantCancelReasonGroupDeleted, participantReason)
}
