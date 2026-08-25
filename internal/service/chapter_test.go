package service_test

import (
	"testing"
	"time"
	"vilib-api/config"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"
	"vilib-api/internal/repository/repository_mocks"
	"vilib-api/internal/service"
	"vilib-api/internal/service/service_mocks"

	"github.com/gojuno/minimock/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// chapterMocks собирает моки, используемые ChapterService.
type chapterMocks struct {
	Access *service_mocks.AccessMock
	Video  *repository_mocks.VideoMock
	Repo   *repository_mocks.ChapterMock
}

func newChapterMocks(mc *minimock.Controller) chapterMocks {
	return chapterMocks{
		Access: service_mocks.NewAccessMock(mc),
		Video:  repository_mocks.NewVideoMock(mc),
		Repo:   repository_mocks.NewChapterMock(mc),
	}
}

func newChapterService(m chapterMocks, cfg config.VideoConfig) *service.ChapterService {
	svc := &service.Service{Access: m.Access}
	return service.NewChapterService(m.Repo, m.Video, svc, cfg)
}

// chapterFixture — общие идентификаторы и конфиг для тестов ChapterService.
type chapterFixture struct {
	AccountID   uuid.UUID
	GroupID     uuid.UUID
	InitiatorID uuid.UUID
	VideoID     uuid.UUID
	ChapterID   uuid.UUID
	Cfg         config.VideoConfig
}

func newChapterFixture() chapterFixture {
	return chapterFixture{
		AccountID:   uuid.New(),
		GroupID:     uuid.New(),
		InitiatorID: uuid.New(),
		VideoID:     uuid.New(),
		ChapterID:   uuid.New(),
		Cfg:         config.VideoConfig{WatchCompletionThreshold: 0.95},
	}
}

// readyVideo возвращает домен видео в статусе ready с известной длительностью — единственный
// статус, у которого можно создавать/двигать главы (Э4-Т3).
func readyVideo(f chapterFixture, durationMs int64) *domain.Video {
	return &domain.Video{
		ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusReady, DurationMs: &durationMs,
	}
}

func TestService_Chapter_List_ForbiddenWithoutWatchRight(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanWatchVideoMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID, f.GroupID).Return(false)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.List(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_List_VideoNotFound_ReturnsNotFound(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanWatchVideoMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID, f.GroupID).Return(true)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(nil, repository.ErrNotFound)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.List(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID)

	require.ErrorIs(t, err, repository.ErrNotFound)
}

func TestService_Chapter_List_VideoBelongsToAnotherGroup_ReturnsForbidden(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanWatchVideoMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID, f.GroupID).Return(true)
	m.Video.SelectMock.
		Expect(minimock.AnyContext, f.VideoID).
		Return(&domain.Video{ID: f.VideoID, GroupID: uuid.New()}, nil)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.List(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_List_EmptyForVideoWithoutChapters(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanWatchVideoMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID, f.GroupID).Return(true)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.SelectProgressByVideoAndUserMock.
		Expect(minimock.AnyContext, f.VideoID, f.InitiatorID, int64(60000)).
		Return(nil, nil)

	svc := newChapterService(m, f.Cfg)

	got, err := svc.List(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID)

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestService_Chapter_List_PassesZeroDurationForVideoWithoutKnownDuration(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanWatchVideoMock.Expect(minimock.AnyContext, f.AccountID, f.InitiatorID, f.GroupID).Return(true)
	m.Video.SelectMock.
		Expect(minimock.AnyContext, f.VideoID).
		Return(&domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusUploading}, nil)
	// Видео без известной длительности (не ready) — главы у него быть не может, значение
	// $duration_ms роли не играет (Э4-Т4): передаём ноль, а не падаем на разыменовании nil.
	m.Repo.SelectProgressByVideoAndUserMock.
		Expect(minimock.AnyContext, f.VideoID, f.InitiatorID, int64(0)).
		Return(nil, nil)

	svc := newChapterService(m, f.Cfg)

	got, err := svc.List(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID)

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestService_Chapter_Create_ForbiddenWithoutManageVideoRight(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.
		Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).
		Return(service.ErrForbidden)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, domain.CreateChapter{StartMs: 0, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_Create_VideoBelongsToAnotherGroup_ReturnsForbidden(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.
		Expect(minimock.AnyContext, f.VideoID).
		Return(&domain.Video{ID: f.VideoID, GroupID: uuid.New()}, nil)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, domain.CreateChapter{StartMs: 0, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_Create_VideoNotReady_ReturnsConflict(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.
		Expect(minimock.AnyContext, f.VideoID).
		Return(&domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusUploading}, nil)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, domain.CreateChapter{StartMs: 0, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrVideoNotReadyForChapters)
}

func TestService_Chapter_Create_StartMsOutOfDurationRange_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 10000), nil)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID,
		domain.CreateChapter{StartMs: 10000, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrChapterStartInvalid)
}

func TestService_Chapter_Create_LimitReached_ReturnsConflict(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.CountByVideoIDMock.Expect(minimock.AnyContext, f.VideoID).Return(100, nil)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID,
		domain.CreateChapter{StartMs: 0, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrChaptersLimit)
}

func TestService_Chapter_Create_DuplicateStartMs_ReturnsConflict(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.CountByVideoIDMock.Expect(minimock.AnyContext, f.VideoID).Return(1, nil)
	m.Repo.InsertMock.
		Expect(minimock.AnyContext, f.VideoID, int64(1000), "Глава").
		Return(domain.Chapter{}, dberrors.VideoChapterErrors.ErrUniqueVideoChaptersVideoIdStartMsKey)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID,
		domain.CreateChapter{StartMs: 1000, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrChapterStartTaken)
}

func TestService_Chapter_Create_Success_ReturnsRecalculatedBound(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	created := domain.Chapter{
		ID: f.ChapterID, VideoID: f.VideoID, StartMs: 1000, Name: "Глава", CreatedAt: time.Now(),
	}
	expectedBound := domain.ChapterBound{Chapter: created, EndMs: 60000}

	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.CountByVideoIDMock.Expect(minimock.AnyContext, f.VideoID).Return(0, nil)
	m.Repo.InsertMock.Expect(minimock.AnyContext, f.VideoID, int64(1000), "Глава").Return(created, nil)
	m.Repo.SelectBoundsByVideoIDMock.
		Expect(minimock.AnyContext, f.VideoID, int64(60000)).
		Return([]domain.ChapterBound{expectedBound}, nil)

	svc := newChapterService(m, f.Cfg)

	got, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID,
		domain.CreateChapter{StartMs: 1000, Name: "Глава"},
	)

	require.NoError(t, err)
	require.Equal(t, expectedBound, got)
}

func TestService_Chapter_Create_ChapterMissingFromBoundsAfterInsert_ReturnsNotFound(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	created := domain.Chapter{ID: f.ChapterID, VideoID: f.VideoID, StartMs: 1000, Name: "Глава"}

	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.CountByVideoIDMock.Expect(minimock.AnyContext, f.VideoID).Return(0, nil)
	m.Repo.InsertMock.Expect(minimock.AnyContext, f.VideoID, int64(1000), "Глава").Return(created, nil)
	m.Repo.SelectBoundsByVideoIDMock.
		Expect(minimock.AnyContext, f.VideoID, int64(60000)).
		Return(nil, nil)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Create(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID,
		domain.CreateChapter{StartMs: 1000, Name: "Глава"},
	)

	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestService_Chapter_Update_ForbiddenWithoutManageVideoRight(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.
		Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).
		Return(service.ErrForbidden)

	svc := newChapterService(m, f.Cfg)

	name := "Новое имя"
	_, err := svc.Update(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID, domain.ChapterPatch{Name: &name},
	)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_Update_ChapterBelongsToAnotherVideo_ReturnsForbidden(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.SelectByIDMock.
		Expect(minimock.AnyContext, f.ChapterID).
		Return(domain.Chapter{ID: f.ChapterID, VideoID: uuid.New()}, nil)

	svc := newChapterService(m, f.Cfg)

	name := "Новое имя"
	_, err := svc.Update(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID, domain.ChapterPatch{Name: &name},
	)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_Update_RenameOnlySkipsRangeValidation(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	existing := domain.Chapter{ID: f.ChapterID, VideoID: f.VideoID, StartMs: 1000, Name: "Старое имя"}
	newName := "Новое имя"
	renamed := existing
	renamed.Name = newName
	expectedBound := domain.ChapterBound{Chapter: renamed, EndMs: 60000}

	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.SelectByIDMock.Expect(minimock.AnyContext, f.ChapterID).Return(existing, nil)
	m.Repo.UpdateMock.
		Expect(minimock.AnyContext, f.ChapterID, domain.ChapterPatch{Name: &newName}).
		Return(renamed, nil)
	m.Repo.SelectBoundsByVideoIDMock.
		Expect(minimock.AnyContext, f.VideoID, int64(60000)).
		Return([]domain.ChapterBound{expectedBound}, nil)

	svc := newChapterService(m, f.Cfg)

	got, err := svc.Update(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID,
		domain.ChapterPatch{Name: &newName},
	)

	require.NoError(t, err)
	require.Equal(t, expectedBound, got)
}

func TestService_Chapter_Update_StartMsOutOfDurationRange_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	existing := domain.Chapter{ID: f.ChapterID, VideoID: f.VideoID, StartMs: 1000, Name: "Глава"}

	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 10000), nil)
	m.Repo.SelectByIDMock.Expect(minimock.AnyContext, f.ChapterID).Return(existing, nil)

	svc := newChapterService(m, f.Cfg)

	newStart := int64(10000)
	_, err := svc.Update(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID,
		domain.ChapterPatch{StartMs: &newStart},
	)

	require.ErrorIs(t, err, service.ErrChapterStartInvalid)
}

func TestService_Chapter_Update_VideoNotReadyForStartChange_ReturnsConflict(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	existing := domain.Chapter{ID: f.ChapterID, VideoID: f.VideoID, StartMs: 1000, Name: "Глава"}

	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.
		Expect(minimock.AnyContext, f.VideoID).
		Return(&domain.Video{ID: f.VideoID, GroupID: f.GroupID, Status: domain.VideoStatusFailed}, nil)
	m.Repo.SelectByIDMock.Expect(minimock.AnyContext, f.ChapterID).Return(existing, nil)

	svc := newChapterService(m, f.Cfg)

	newStart := int64(2000)
	_, err := svc.Update(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID,
		domain.ChapterPatch{StartMs: &newStart},
	)

	require.ErrorIs(t, err, service.ErrVideoNotReadyForChapters)
}

func TestService_Chapter_Update_DuplicateStartMs_ReturnsConflict(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	existing := domain.Chapter{ID: f.ChapterID, VideoID: f.VideoID, StartMs: 1000, Name: "Глава"}

	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.SelectByIDMock.Expect(minimock.AnyContext, f.ChapterID).Return(existing, nil)
	newStart := int64(5000)
	m.Repo.UpdateMock.
		Expect(minimock.AnyContext, f.ChapterID, domain.ChapterPatch{StartMs: &newStart}).
		Return(domain.Chapter{}, dberrors.VideoChapterErrors.ErrUniqueVideoChaptersVideoIdStartMsKey)

	svc := newChapterService(m, f.Cfg)

	_, err := svc.Update(
		t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID,
		domain.ChapterPatch{StartMs: &newStart},
	)

	require.ErrorIs(t, err, service.ErrChapterStartTaken)
}

func TestService_Chapter_Delete_ForbiddenWithoutManageVideoRight(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.
		Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).
		Return(service.ErrForbidden)

	svc := newChapterService(m, f.Cfg)

	err := svc.Delete(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_Delete_ChapterBelongsToAnotherVideo_ReturnsForbidden(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.SelectByIDMock.
		Expect(minimock.AnyContext, f.ChapterID).
		Return(domain.Chapter{ID: f.ChapterID, VideoID: uuid.New()}, nil)

	svc := newChapterService(m, f.Cfg)

	err := svc.Delete(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID)

	require.ErrorIs(t, err, service.ErrForbidden)
}

func TestService_Chapter_Delete_Success(t *testing.T) {
	t.Parallel()

	f := newChapterFixture()
	mc := minimock.NewController(t)
	m := newChapterMocks(mc)
	m.Access.CanManageVideoMock.Expect(minimock.AnyContext, f.AccountID, f.GroupID, f.InitiatorID).Return(nil)
	m.Video.SelectMock.Expect(minimock.AnyContext, f.VideoID).Return(readyVideo(f, 60000), nil)
	m.Repo.SelectByIDMock.
		Expect(minimock.AnyContext, f.ChapterID).
		Return(domain.Chapter{ID: f.ChapterID, VideoID: f.VideoID}, nil)
	m.Repo.DeleteMock.Expect(minimock.AnyContext, f.ChapterID).Return(nil)

	svc := newChapterService(m, f.Cfg)

	err := svc.Delete(t.Context(), f.AccountID, f.GroupID, f.InitiatorID, f.VideoID, f.ChapterID)

	require.NoError(t, err)
}
