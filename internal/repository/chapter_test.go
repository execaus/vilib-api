package repository_test

import (
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

// newTestReadyVideo создаёт готовое видео с известной длительностью — главы можно создавать
// только у видео в статусе ready (Э4-Т3).
func newTestReadyVideo(t *testing.T, r *repository.Repository, f faker.Faker, durationMs int64) domain.Video {
	t.Helper()

	video := newTestVideo(t, r, f, domain.VideoStatusReady)

	updated, err := r.Video.UpdateStatusIf(
		t.Context(),
		video.ID,
		[]domain.VideoStatus{domain.VideoStatusReady},
		domain.VideoStatusReady,
		domain.VideoPatch{DurationMs: &durationMs},
	)
	require.NoError(t, err)
	require.True(t, updated)

	got, err := r.Video.Select(t.Context(), video.ID)
	require.NoError(t, err)

	return *got
}

func TestRepository_ChapterInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 60000)

		chapter, err := r.Chapter.Insert(t.Context(), video.ID, 1000, "Введение")

		require.NoError(t, err)
		require.NotEmpty(t, chapter.ID)
		require.Equal(t, video.ID, chapter.VideoID)
		require.Equal(t, int64(1000), chapter.StartMs)
		require.Equal(t, "Введение", chapter.Name)
		require.WithinDuration(t, time.Now(), chapter.CreatedAt, time.Minute)
	})
}

func TestRepository_ChapterInsert_DuplicateStartMs_ReturnsUniqueConstraintError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 60000)

		_, err := r.Chapter.Insert(t.Context(), video.ID, 1000, "Первая")
		require.NoError(t, err)

		_, err = r.Chapter.Insert(t.Context(), video.ID, 1000, "Дубль")

		require.Error(t, err)
		require.ErrorIs(t, dberrors.VideoChapterErrors.ErrUniqueVideoChaptersVideoIdStartMsKey, err)
	})
}

func TestRepository_ChapterSelectByID_NotFound_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		_, err := r.Chapter.SelectByID(t.Context(), uuid.New())

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_ChapterSelectBoundsByVideoID_ComputesEndMsFromNextChapterStartAndDuration(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		first, err := r.Chapter.Insert(t.Context(), video.ID, 0, "Первая")
		require.NoError(t, err)
		second, err := r.Chapter.Insert(t.Context(), video.ID, 10000, "Вторая")
		require.NoError(t, err)
		third, err := r.Chapter.Insert(t.Context(), video.ID, 20000, "Третья")
		require.NoError(t, err)

		bounds, err := r.Chapter.SelectBoundsByVideoID(t.Context(), video.ID, 30000)

		require.NoError(t, err)
		require.Len(t, bounds, 3)
		require.Equal(t, first.ID, bounds[0].ID)
		require.Equal(t, int64(10000), bounds[0].EndMs)
		require.Equal(t, second.ID, bounds[1].ID)
		require.Equal(t, int64(20000), bounds[1].EndMs)
		require.Equal(t, third.ID, bounds[2].ID)
		require.Equal(t, int64(30000), bounds[2].EndMs, "конец последней главы — длительность видео")
	})
}

func TestRepository_ChapterSelectBoundsByVideoID_EmptyForVideoWithoutChapters(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		bounds, err := r.Chapter.SelectBoundsByVideoID(t.Context(), video.ID, 30000)

		require.NoError(t, err)
		require.Empty(t, bounds)
	})
}

func TestRepository_ChapterSelectProgressByVideoAndUser_ComputesCoveredMsFromIntersection(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)
		userID := video.Author
		now := time.Now().UTC().Truncate(time.Millisecond)

		chapterOne, err := r.Chapter.Insert(t.Context(), video.ID, 0, "Первая")
		require.NoError(t, err)
		_, err = r.Chapter.Insert(t.Context(), video.ID, 10000, "Вторая")
		require.NoError(t, err)

		// Интервал просмотра [5000, 15000) пересекает первую главу на 5000мс, вторую — на 5000мс.
		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userID, video.ID, now))
		hugeNeedMs := int64(1) << 40
		_, err = r.WatchProgress.Apply(t.Context(), userID, video.ID, 5000, 15000, 15000, 10000, now, hugeNeedMs)
		require.NoError(t, err)

		progress, err := r.Chapter.SelectProgressByVideoAndUser(t.Context(), video.ID, userID, 30000)

		require.NoError(t, err)
		require.Len(t, progress, 2)
		require.Equal(t, chapterOne.ID, progress[0].ID)
		require.Equal(t, int64(5000), progress[0].CoveredMs)
		require.Equal(t, int64(5000), progress[1].CoveredMs)
	})
}

func TestRepository_ChapterSelectProgressByVideoAndUser_ZeroCoverageWithoutWatchProgress(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		_, err := r.Chapter.Insert(t.Context(), video.ID, 0, "Первая")
		require.NoError(t, err)

		progress, err := r.Chapter.SelectProgressByVideoAndUser(t.Context(), video.ID, uuid.New(), 30000)

		require.NoError(t, err)
		require.Len(t, progress, 1)
		require.Equal(t, int64(0), progress[0].CoveredMs)
	})
}

func TestRepository_ChapterSelectProgressByVideoAndUsers_BatchAcrossManyUsers(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)
		watchedUser := video.Author
		unwatchedUser := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)

		chapter, err := r.Chapter.Insert(t.Context(), video.ID, 0, "Первая")
		require.NoError(t, err)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), watchedUser, video.ID, now))
		hugeNeedMs := int64(1) << 40
		_, err = r.WatchProgress.Apply(t.Context(), watchedUser, video.ID, 0, 10000, 10000, 10000, now, hugeNeedMs)
		require.NoError(t, err)

		progress, err := r.Chapter.SelectProgressByVideoAndUsers(
			t.Context(), video.ID, []uuid.UUID{watchedUser, unwatchedUser}, 30000,
		)

		require.NoError(t, err)
		// unwatchedUser не имеет строки watch_progress — не участвует в выборке (§3 дизайна
		// эпика Э4): вызывающая сторона трактует отсутствие как нулевое покрытие без обращения
		// к БД.
		require.Len(t, progress, 1)
		require.Equal(t, watchedUser, progress[0].UserID)
		require.Equal(t, chapter.ID, progress[0].ID)
		require.Equal(t, int64(10000), progress[0].CoveredMs)
	})
}

func TestRepository_ChapterSelectProgressByVideoAndUsers_EmptyUserIDsReturnsNilWithoutError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		progress, err := r.Chapter.SelectProgressByVideoAndUsers(t.Context(), video.ID, nil, 30000)

		require.NoError(t, err)
		require.Nil(t, progress)
	})
}

func TestRepository_ChapterUpdate_ChangesStartAndName(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		chapter, err := r.Chapter.Insert(t.Context(), video.ID, 1000, "Черновое имя")
		require.NoError(t, err)

		newStart := int64(2000)
		newName := "Итоговое имя"
		updated, err := r.Chapter.Update(
			t.Context(),
			chapter.ID,
			domain.ChapterPatch{StartMs: &newStart, Name: &newName},
		)

		require.NoError(t, err)
		require.Equal(t, newStart, updated.StartMs)
		require.Equal(t, newName, updated.Name)
	})
}

func TestRepository_ChapterUpdate_ConflictOnStartMs_ReturnsUniqueConstraintError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		_, err := r.Chapter.Insert(t.Context(), video.ID, 1000, "Первая")
		require.NoError(t, err)
		second, err := r.Chapter.Insert(t.Context(), video.ID, 2000, "Вторая")
		require.NoError(t, err)

		conflictStart := int64(1000)
		_, err = r.Chapter.Update(t.Context(), second.ID, domain.ChapterPatch{StartMs: &conflictStart})

		require.Error(t, err)
		require.ErrorIs(t, dberrors.VideoChapterErrors.ErrUniqueVideoChaptersVideoIdStartMsKey, err)
	})
}

func TestRepository_ChapterUpdate_NotFound_ReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		name := "Имя"

		_, err := r.Chapter.Update(t.Context(), uuid.New(), domain.ChapterPatch{Name: &name})

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_ChapterDelete_RemovesRowAndNeighborInheritsGap(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		first, err := r.Chapter.Insert(t.Context(), video.ID, 0, "Первая")
		require.NoError(t, err)
		middle, err := r.Chapter.Insert(t.Context(), video.ID, 10000, "Средняя")
		require.NoError(t, err)
		_, err = r.Chapter.Insert(t.Context(), video.ID, 20000, "Третья")
		require.NoError(t, err)

		require.NoError(t, r.Chapter.Delete(t.Context(), middle.ID))

		_, err = r.Chapter.SelectByID(t.Context(), middle.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)

		bounds, err := r.Chapter.SelectBoundsByVideoID(t.Context(), video.ID, 30000)
		require.NoError(t, err)
		require.Len(t, bounds, 2)
		require.Equal(t, first.ID, bounds[0].ID)
		require.Equal(t, int64(20000), bounds[0].EndMs, "конец первой главы — начало третьей, средняя удалена")
	})
}

func TestRepository_ChapterDelete_CascadesFromVideoDeletion(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestReadyVideo(t, r, f, 30000)

		chapter, err := r.Chapter.Insert(t.Context(), video.ID, 0, "Первая")
		require.NoError(t, err)

		require.NoError(t, r.Video.Delete(t.Context(), video.ID))

		_, err = r.Chapter.SelectByID(t.Context(), chapter.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_ChapterCountByVideoID_CountsOnlyMatchingVideo(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		videoA := newTestReadyVideo(t, r, f, 30000)
		videoB := newTestReadyVideo(t, r, f, 30000)

		_, err := r.Chapter.Insert(t.Context(), videoA.ID, 0, "A1")
		require.NoError(t, err)
		_, err = r.Chapter.Insert(t.Context(), videoA.ID, 1000, "A2")
		require.NoError(t, err)
		_, err = r.Chapter.Insert(t.Context(), videoB.ID, 0, "B1")
		require.NoError(t, err)

		count, err := r.Chapter.CountByVideoID(t.Context(), videoA.ID)

		require.NoError(t, err)
		require.Equal(t, 2, count)
	})
}
