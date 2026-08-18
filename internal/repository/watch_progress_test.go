package repository_test

import (
	"context"
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/internal/saga"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

// testHugeNeedMs — сентинел «длительность видео ещё не известна» для тестов Apply/
// OnDurationKnown, не зависящий от внутренней константы сервиса.
const testHugeNeedMs = int64(1) << 40

func TestRepository_WatchProgressApply_MergesOverlappingIntervals(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		userID := video.Author
		now := time.Now().UTC().Truncate(time.Millisecond)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userID, video.ID, now))

		_, err := r.WatchProgress.Apply(t.Context(), userID, video.ID, 0, 10000, 10000, 10000, now, testHugeNeedMs)
		require.NoError(t, err)

		progress, err := r.WatchProgress.Apply(
			t.Context(), userID, video.ID, 5000, 20000, 20000, 15000, now.Add(time.Second), testHugeNeedMs,
		)
		require.NoError(t, err)

		require.Equal(t, int64(20000), progress.CoveredMs)
		require.Equal(t, int64(20000), progress.LastPositionMs)
		require.Equal(t, int64(25000), progress.WallMs)
	})
}

func TestRepository_WatchProgressApply_KeepsDisjointIntervalsSeparate(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		userID := video.Author
		now := time.Now().UTC().Truncate(time.Millisecond)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userID, video.ID, now))

		_, err := r.WatchProgress.Apply(t.Context(), userID, video.ID, 0, 10, 10, 10, now, testHugeNeedMs)
		require.NoError(t, err)

		progress, err := r.WatchProgress.Apply(
			t.Context(), userID, video.ID, 20, 30, 30, 10, now.Add(time.Second), testHugeNeedMs,
		)
		require.NoError(t, err)

		require.Equal(t, int64(20), progress.CoveredMs)
	})
}

func TestRepository_WatchProgressApply_SetsThresholdReachedAtOnceOnFirstCrossing(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		userID := video.Author
		firstNow := time.Now().UTC().Truncate(time.Millisecond)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userID, video.ID, firstNow))

		// need=1000: интервал [0,1000) сразу пересекает порог.
		first, err := r.WatchProgress.Apply(t.Context(), userID, video.ID, 0, 1000, 1000, 1000, firstNow, 1000)
		require.NoError(t, err)
		require.NotNil(t, first.ThresholdReachedAt)
		require.WithinDuration(t, firstNow, *first.ThresholdReachedAt, time.Second)

		// Повторный heartbeat с более поздним now не должен переписать момент первого
		// достижения порога (coalesce в Apply).
		secondNow := firstNow.Add(time.Hour)
		second, err := r.WatchProgress.Apply(t.Context(), userID, video.ID, 1000, 2000, 2000, 1000, secondNow, 1000)
		require.NoError(t, err)
		require.NotNil(t, second.ThresholdReachedAt)
		require.Equal(t, first.ThresholdReachedAt.UTC(), second.ThresholdReachedAt.UTC())
	})
}

func TestRepository_WatchProgressOnDurationKnown_ReturnsOnlyUsersCrossingThresholdFirstTime(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		userBelow := video.Author

		userAtOrAbove := newTestVideo(t, r, f, domain.VideoStatusReady).Author
		userAlreadyReached := newTestVideo(t, r, f, domain.VideoStatusReady).Author

		now := time.Now().UTC().Truncate(time.Millisecond)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userBelow, video.ID, now))
		_, err := r.WatchProgress.Apply(t.Context(), userBelow, video.ID, 0, 100, 100, 100, now, testHugeNeedMs)
		require.NoError(t, err)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userAtOrAbove, video.ID, now))
		_, err = r.WatchProgress.Apply(t.Context(), userAtOrAbove, video.ID, 0, 1000, 1000, 1000, now, testHugeNeedMs)
		require.NoError(t, err)

		require.NoError(t, r.WatchProgress.InsertEmpty(t.Context(), userAlreadyReached, video.ID, now))
		_, err = r.WatchProgress.Apply(t.Context(), userAlreadyReached, video.ID, 0, 1000, 1000, 1000, now, 1000)
		require.NoError(t, err)

		completedNow := now.Add(time.Hour)
		userIDs, err := r.WatchProgress.OnDurationKnown(t.Context(), video.ID, 1000, completedNow)
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{userAtOrAbove}, userIDs)

		reached, err := r.WatchProgress.SelectByUserAndVideoIDs(t.Context(), userAtOrAbove, []uuid.UUID{video.ID})
		require.NoError(t, err)
		require.Len(t, reached, 1)
		require.NotNil(t, reached[0].ThresholdReachedAt)
		require.WithinDuration(t, completedNow, *reached[0].ThresholdReachedAt, time.Second)
	})
}

// TestRepository_WatchProgressSelectForUpdate_LocksRowWithinTransaction проверяет, что
// SelectForUpdate не падает внутри явной транзакции и её можно снять коммитом/откатом —
// первый шаг сериализации heartbeat'ов одной пары (user, video), §3 шаг 2 дизайна эпика Э3.
func TestRepository_WatchProgressSelectForUpdate_LocksRowWithinTransaction(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		repo := repository.NewRepository(provider)
		f := testutil.Faker

		video := newTestVideo(t, repo, f, domain.VideoStatusReady)
		userID := video.Author
		now := time.Now().UTC().Truncate(time.Millisecond)

		require.NoError(t, repo.WatchProgress.InsertEmpty(t.Context(), userID, video.ID, now))

		tx1, err := provider.WithTx(t.Context())
		require.NoError(t, err)
		ctx1 := context.WithValue(t.Context(), saga.CtxKey, tx1)

		_, err = repo.WatchProgress.SelectForUpdate(ctx1, userID, video.ID)
		require.NoError(t, err)
		require.NoError(t, tx1.Rollback(t.Context()))

		tx2, err := provider.WithTx(t.Context())
		require.NoError(t, err)
		ctx2 := context.WithValue(t.Context(), saga.CtxKey, tx2)

		_, err = repo.WatchProgress.SelectForUpdate(ctx2, userID, video.ID)
		require.NoError(t, err)
		require.NoError(t, tx2.Rollback(t.Context()))
	})
}
