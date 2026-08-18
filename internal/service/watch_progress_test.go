package service_test

import (
	"math"
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

// watchProgressMocks собирает моки, используемые WatchProgressService.
type watchProgressMocks struct {
	Access       *service_mocks.AccessMock
	Video        *repository_mocks.VideoMock
	Progress     *repository_mocks.WatchProgressMock
	Sessions     *repository_mocks.WatchSessionMock
	Participants *repository_mocks.AssignmentParticipantMock
}

func newWatchProgressMocks(mc *minimock.Controller) watchProgressMocks {
	return watchProgressMocks{
		Access:       service_mocks.NewAccessMock(mc),
		Video:        repository_mocks.NewVideoMock(mc),
		Progress:     repository_mocks.NewWatchProgressMock(mc),
		Sessions:     repository_mocks.NewWatchSessionMock(mc),
		Participants: repository_mocks.NewAssignmentParticipantMock(mc),
	}
}

func newWatchProgressService(
	m watchProgressMocks, cfg config.VideoConfig, now time.Time,
) *service.WatchProgressService {
	svc := &service.Service{Access: m.Access}
	return service.NewWatchProgressService(
		m.Progress, m.Sessions, m.Participants, m.Video, svc, cfg,
		service.WithWatchProgressNow(func() time.Time { return now }),
	)
}

// watchProgressFixture — общие идентификаторы и конфиг для тестов WatchProgressService.
type watchProgressFixture struct {
	AccountID uuid.UUID
	GroupID   uuid.UUID
	UserID    uuid.UUID
	VideoID   uuid.UUID
	SessionID uuid.UUID
	Now       time.Time
	Cfg       config.VideoConfig
}

func newWatchProgressFixture() watchProgressFixture {
	return watchProgressFixture{
		AccountID: uuid.New(),
		GroupID:   uuid.New(),
		UserID:    uuid.New(),
		VideoID:   uuid.New(),
		SessionID: uuid.New(),
		Now:       time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC),
		Cfg:       config.VideoConfig{WatchCompletionThreshold: 0.95, WatchHeartbeatInterval: 10 * time.Second},
	}
}

func TestService_WatchProgress_Heartbeat_Forbidden(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	m.Access.CanWatchVideoMock.Expect(minimock.AnyContext, f.AccountID, f.UserID, f.GroupID).Return(false)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	_, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 1, ToMs: 1000, PositionMs: 1000, PlaybackRate: 1,
	})

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_WatchProgress_Heartbeat_VideoNotAvailable(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(&domain.Video{
		ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusUploading,
	}, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	_, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 1, ToMs: 1000, PositionMs: 1000, PlaybackRate: 1,
	})

	require.ErrorIs(t, err, service.ErrVideoNotAvailable)
}

func TestService_WatchProgress_Heartbeat_IdempotentOnSeqReplay(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	video := domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady}
	progress := domain.WatchProgress{
		UserID: f.UserID, VideoID: f.VideoID, CoveredMs: 4000, LastPositionMs: 4000, LastAt: f.Now,
	}
	session := domain.WatchSession{
		SessionID: f.SessionID, UserID: f.UserID, VideoID: f.VideoID, LastSeq: 5, LastAt: f.Now,
	}

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(&video, nil)
	m.Progress.SelectForUpdateMock.Expect(minimock.AnyContext, f.UserID, f.VideoID).Return(progress, nil)
	m.Sessions.SelectForUpdateMock.Expect(minimock.AnyContext, f.SessionID).Return(session, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	state, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 5, FromMs: 4000, ToMs: 14000, PositionMs: 14000, PlaybackRate: 1,
	})

	// Apply/UpdatePosition/Sessions.Update намеренно не заэкспектированы — повторный (не более
	// нового) seq не должен вызывать ни одного из них (Э3-Т10).
	require.NoError(t, err)
	require.False(t, state.Accepted)
	require.Equal(t, progress.CoveredMs, state.CoveredMs)
	require.Equal(t, progress.LastPositionMs, state.LastPositionMs)
}

func TestService_WatchProgress_Heartbeat_ForeignSessionForbidden(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	video := domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady}
	progress := domain.WatchProgress{UserID: f.UserID, VideoID: f.VideoID, LastAt: f.Now}
	otherUsersSession := domain.WatchSession{
		SessionID: f.SessionID, UserID: uuid.New(), VideoID: f.VideoID, LastSeq: 1, LastAt: f.Now,
	}

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Return(&video, nil)
	m.Progress.SelectForUpdateMock.Return(progress, nil)
	m.Sessions.SelectForUpdateMock.Return(otherUsersSession, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	_, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 2, FromMs: 0, ToMs: 1000, PositionMs: 1000, PlaybackRate: 1,
	})

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_WatchProgress_Heartbeat_RateAboveOneDiscardsInterval(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	video := domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady}
	progress := domain.WatchProgress{UserID: f.UserID, VideoID: f.VideoID, LastAt: f.Now}
	session := domain.WatchSession{
		SessionID: f.SessionID, UserID: f.UserID, VideoID: f.VideoID, LastSeq: 1, LastAt: f.Now,
	}

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Return(&video, nil)
	m.Progress.SelectForUpdateMock.Return(progress, nil)
	m.Sessions.SelectForUpdateMock.Return(session, nil)
	m.Progress.UpdatePositionMock.Expect(minimock.AnyContext, f.UserID, f.VideoID, int64(5000), f.Now).
		Return(domain.WatchProgress{UserID: f.UserID, VideoID: f.VideoID, LastPositionMs: 5000}, nil)
	m.Sessions.UpdateMock.Expect(minimock.AnyContext, f.SessionID, int64(2), f.Now, int64(5000)).
		Return(domain.WatchSession{}, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	state, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 2, FromMs: 0, ToMs: 5000, PositionMs: 5000, PlaybackRate: 1.25,
	})

	require.NoError(t, err)
	require.False(t, state.Accepted)
	// Apply/участники намеренно не заэкспектированы — интервал с rate > 1.0 отброшен целиком
	// (В-1 решение владельца).
}

func TestService_WatchProgress_Heartbeat_TruncatesRewindBeyondElapsedTolerance(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	video := domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady}
	// Оба last_at (прогресса и сессии) — 5 секунд назад: elapsed = 5000мс, допуск ×1.1 = 5500мс.
	progressLastAt := f.Now.Add(-5 * time.Second)
	progress := domain.WatchProgress{UserID: f.UserID, VideoID: f.VideoID, LastAt: progressLastAt}
	session := domain.WatchSession{
		SessionID: f.SessionID, UserID: f.UserID, VideoID: f.VideoID, LastSeq: 1, LastAt: progressLastAt,
	}

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Return(&video, nil)
	m.Progress.SelectForUpdateMock.Return(progress, nil)
	m.Sessions.SelectForUpdateMock.Return(session, nil)

	// Клиент прислал 20 секунд «прыжком» — должно урезаться до 5500мс (elapsed×1.1).
	m.Progress.ApplyMock.
		Expect(
			minimock.AnyContext,
			f.UserID,
			f.VideoID,
			int64(0),
			int64(5500),
			int64(20000),
			int64(5000),
			f.Now,
			int64(math.MaxInt64),
		).
		Return(domain.WatchProgress{UserID: f.UserID, VideoID: f.VideoID, CoveredMs: 5500, LastPositionMs: 20000}, nil)
	m.Sessions.UpdateMock.Return(domain.WatchSession{}, nil)
	m.Participants.UpdateStatusByUserVideoMock.
		Expect(
			minimock.AnyContext, f.UserID, f.VideoID,
			domain.AssignmentParticipantStatusAssigned, domain.AssignmentParticipantStatusInProgress,
		).
		Return(nil, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	state, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 2, FromMs: 0, ToMs: 20000, PositionMs: 20000, PlaybackRate: 1,
	})

	require.NoError(t, err)
	require.True(t, state.Accepted)
	require.Equal(t, int64(5500), state.CoveredMs)
}

func TestService_WatchProgress_Heartbeat_WallCapLimitsTwoSessionsCoverage(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	video := domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady}
	// Прогресс уже накоплен первой вкладкой: wall_ms=1000, covered_ms=1000 (ratio ровно 1.0).
	// last_at прогресса — 2 секунды назад: wall_delta = min(accepted, 2000) = 2000, allowance =
	// (1000+2000)×1.1 − 1000 = 2300 — второй вкладке достаётся не весь присланный интервал.
	progressLastAt := f.Now.Add(-2 * time.Second)
	progress := domain.WatchProgress{
		UserID: f.UserID, VideoID: f.VideoID, WallMs: 1000, CoveredMs: 1000, LastAt: progressLastAt,
	}
	otherSessionID := uuid.New()

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Return(&video, nil)
	m.Progress.SelectForUpdateMock.Return(progress, nil)
	// Новая (вторая) сессия — elapsed для усечения перемотки берётся из HeartbeatInterval.
	m.Sessions.SelectForUpdateMock.Expect(minimock.AnyContext, otherSessionID).
		Return(domain.WatchSession{}, repository.ErrNotFound)
	m.Sessions.InsertMock.Return(
		domain.WatchSession{SessionID: otherSessionID, UserID: f.UserID, VideoID: f.VideoID},
		nil,
	)

	m.Progress.ApplyMock.
		Expect(
			minimock.AnyContext,
			f.UserID,
			f.VideoID,
			int64(2000),
			int64(4300),
			int64(8000),
			int64(2000),
			f.Now,
			int64(math.MaxInt64),
		).
		Return(domain.WatchProgress{UserID: f.UserID, VideoID: f.VideoID, CoveredMs: 3300, LastPositionMs: 8000}, nil)
	m.Sessions.UpdateMock.Return(domain.WatchSession{}, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	state, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: otherSessionID, Seq: 1, FromMs: 2000, ToMs: 10000, PositionMs: 8000, PlaybackRate: 1,
	})

	require.NoError(t, err)
	require.True(t, state.Accepted)
	require.Equal(t, int64(3300), state.CoveredMs)
}

func TestService_WatchProgress_Heartbeat_CompletesParticipantsOnceOnThresholdCrossing(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	durationMs := int64(10000)
	video := domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady, DurationMs: &durationMs}
	// last_at (прогресса и сессии) — 2 секунды назад, wall_ms/covered_ms уже большие (прошлый
	// длинный просмотр) — интервал принимается без усечения (elapsed×1.1 и wall-cap с запасом).
	lastAt := f.Now.Add(-2 * time.Second)
	progress := domain.WatchProgress{
		UserID: f.UserID, VideoID: f.VideoID, CoveredMs: 8000, WallMs: 8000, LastAt: lastAt,
	}
	session := domain.WatchSession{
		SessionID: f.SessionID, UserID: f.UserID, VideoID: f.VideoID, LastSeq: 1, LastAt: lastAt,
	}

	m.Access.CanWatchVideoMock.Return(true)
	m.Video.SelectMock.Return(&video, nil)
	m.Progress.SelectForUpdateMock.Return(progress, nil)
	m.Sessions.SelectForUpdateMock.Return(session, nil)

	completedAt := f.Now
	m.Progress.ApplyMock.Return(domain.WatchProgress{
		UserID: f.UserID, VideoID: f.VideoID, CoveredMs: 9500, LastPositionMs: 9500,
		ThresholdReachedAt: &completedAt,
	}, nil)
	m.Sessions.UpdateMock.Return(domain.WatchSession{}, nil)
	m.Participants.CompleteByUserVideoMock.
		Expect(minimock.AnyContext, f.UserID, f.VideoID, f.Now, 95, 95, &f.SessionID).
		Return([]uuid.UUID{uuid.New()}, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	state, err := svc.Heartbeat(t.Context(), f.AccountID, f.GroupID, f.UserID, f.VideoID, domain.Heartbeat{
		SessionID: f.SessionID, Seq: 2, FromMs: 8000, ToMs: 9500, PositionMs: 9500, PlaybackRate: 1,
	})

	require.NoError(t, err)
	require.True(t, state.Completed)
	require.Equal(t, uint64(1), m.Participants.CompleteByUserVideoAfterCounter())
}

func TestService_WatchProgress_OnDurationKnown_CompletesAccumulatedProgress(t *testing.T) {
	t.Parallel()

	f := newWatchProgressFixture()
	mc := minimock.NewController(t)
	m := newWatchProgressMocks(mc)

	durationMs := int64(10000)
	needMs := int64(9500)

	m.Progress.OnDurationKnownMock.
		Expect(minimock.AnyContext, f.VideoID, needMs, f.Now).
		Return([]uuid.UUID{f.UserID}, nil)
	m.Progress.SelectByUserAndVideoIDsMock.
		Expect(minimock.AnyContext, f.UserID, []uuid.UUID{f.VideoID}).
		Return([]domain.WatchProgress{{UserID: f.UserID, VideoID: f.VideoID, CoveredMs: 9700}}, nil)
	m.Participants.CompleteByUserVideoMock.
		Expect(minimock.AnyContext, f.UserID, f.VideoID, f.Now, 97, 95, (*uuid.UUID)(nil)).
		Return([]uuid.UUID{uuid.New()}, nil)

	svc := newWatchProgressService(m, f.Cfg, f.Now)
	err := svc.OnDurationKnown(t.Context(), f.VideoID, durationMs)

	require.NoError(t, err)
}
