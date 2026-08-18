package repository_test

import (
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRepository_WatchSessionInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		sessionID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)

		session, err := r.WatchSession.Insert(t.Context(), sessionID, video.Author, video.ID, now, 0)

		require.NoError(t, err)
		require.Equal(t, sessionID, session.SessionID)
		require.Equal(t, video.Author, session.UserID)
		require.Equal(t, video.ID, session.VideoID)
		require.Zero(t, session.LastSeq)
	})
}

func TestRepository_WatchSessionSelectForUpdate_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		_, err := r.WatchSession.SelectForUpdate(t.Context(), uuid.New())

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_WatchSessionUpdate_AdvancesSeqAndPosition(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		sessionID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)

		_, err := r.WatchSession.Insert(t.Context(), sessionID, video.Author, video.ID, now, 0)
		require.NoError(t, err)

		later := now.Add(10 * time.Second)
		session, err := r.WatchSession.Update(t.Context(), sessionID, 1, later, 10000)

		require.NoError(t, err)
		require.EqualValues(t, 1, session.LastSeq)
		require.Equal(t, int64(10000), session.LastPositionMs)
		require.WithinDuration(t, later, session.LastAt, time.Second)
	})
}

func TestRepository_WatchSessionDeleteOlderThan_RemovesOnlyStaleSessions(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		now := time.Now().UTC().Truncate(time.Millisecond)

		staleID := uuid.New()
		freshID := uuid.New()

		_, err := r.WatchSession.Insert(t.Context(), staleID, video.Author, video.ID, now.Add(-40*24*time.Hour), 0)
		require.NoError(t, err)
		_, err = r.WatchSession.Insert(t.Context(), freshID, video.Author, video.ID, now, 0)
		require.NoError(t, err)

		deleted, err := r.WatchSession.DeleteOlderThan(t.Context(), now.Add(-30*24*time.Hour))

		require.NoError(t, err)
		require.EqualValues(t, 1, deleted)

		_, err = r.WatchSession.SelectForUpdate(t.Context(), staleID)
		require.ErrorIs(t, err, repository.ErrNotFound)

		_, err = r.WatchSession.SelectForUpdate(t.Context(), freshID)
		require.NoError(t, err)
	})
}
