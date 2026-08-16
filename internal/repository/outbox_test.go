package repository_test

import (
	"context"
	"testing"
	"vilib-api/internal/repository"
	"vilib-api/internal/saga"
	"vilib-api/testutil"

	"github.com/jaswdr/faker/v2"
	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

func TestRepository_OutboxInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		err := r.Outbox.Insert(t.Context(), "video.original-uploaded", "video-1", []byte(`{"a":1}`))

		require.NoError(t, err)

		events, selectErr := r.Outbox.SelectBatchForUpdate(t.Context(), 10)

		require.NoError(t, selectErr)
		require.Len(t, events, 1)
		require.Equal(t, "video.original-uploaded", events[0].Topic)
		require.Equal(t, "video-1", events[0].Key)
		require.JSONEq(t, `{"a":1}`, string(events[0].Payload))
		require.NotZero(t, events[0].ID)
		require.NotZero(t, events[0].CreatedAt)
	})
}

func TestRepository_OutboxSelectBatchForUpdate_OrderedByID(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		for range 3 {
			require.NoError(t, r.Outbox.Insert(t.Context(), "topic", "key", []byte(`{}`)))
		}

		events, err := r.Outbox.SelectBatchForUpdate(t.Context(), 10)

		require.NoError(t, err)
		require.Len(t, events, 3)
		require.Less(t, events[0].ID, events[1].ID)
		require.Less(t, events[1].ID, events[2].ID)
	})
}

func TestRepository_OutboxSelectBatchForUpdate_RespectsLimit(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		for range 5 {
			require.NoError(t, r.Outbox.Insert(t.Context(), "topic", "key", []byte(`{}`)))
		}

		events, err := r.Outbox.SelectBatchForUpdate(t.Context(), 2)

		require.NoError(t, err)
		require.Len(t, events, 2)
	})
}

func TestRepository_OutboxSelectBatchForUpdate_EmptyWhenNoRows(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		events, err := r.Outbox.SelectBatchForUpdate(t.Context(), 10)

		require.NoError(t, err)
		require.Empty(t, events)
	})
}

func TestRepository_OutboxDeleteByIDs_RemovesOnlySpecifiedRows(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		require.NoError(t, r.Outbox.Insert(t.Context(), "topic", "key-1", []byte(`{}`)))
		require.NoError(t, r.Outbox.Insert(t.Context(), "topic", "key-2", []byte(`{}`)))

		events, err := r.Outbox.SelectBatchForUpdate(t.Context(), 10)
		require.NoError(t, err)
		require.Len(t, events, 2)

		err = r.Outbox.DeleteByIDs(t.Context(), []int64{events[0].ID})
		require.NoError(t, err)

		remaining, err := r.Outbox.SelectBatchForUpdate(t.Context(), 10)
		require.NoError(t, err)
		require.Len(t, remaining, 1)
		require.Equal(t, events[1].ID, remaining[0].ID)
	})
}

func TestRepository_OutboxDeleteByIDs_EmptySliceNoop(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		require.NoError(t, r.Outbox.Insert(t.Context(), "topic", "key", []byte(`{}`)))

		err := r.Outbox.DeleteByIDs(t.Context(), nil)
		require.NoError(t, err)

		events, selectErr := r.Outbox.SelectBatchForUpdate(t.Context(), 10)
		require.NoError(t, selectErr)
		require.Len(t, events, 1)
	})
}

// TestRepository_OutboxSelectBatchForUpdate_SkipLockedAvoidsOverlap проверяет, что вторая
// параллельная транзакция не получает строки, уже заблокированные первой (FOR UPDATE SKIP
// LOCKED) — это делает релей безопасным при нескольких инстансах API.
func TestRepository_OutboxSelectBatchForUpdate_SkipLockedAvoidsOverlap(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		repo := repository.NewRepository(provider)

		for range 4 {
			require.NoError(t, repo.Outbox.Insert(t.Context(), "topic", "key", []byte(`{}`)))
		}

		tx1, err := provider.WithTx(t.Context())
		require.NoError(t, err)
		ctx1 := context.WithValue(t.Context(), saga.CtxKey, tx1)

		batch1, err := repo.Outbox.SelectBatchForUpdate(ctx1, 2)
		require.NoError(t, err)
		require.Len(t, batch1, 2)

		tx2, err := provider.WithTx(t.Context())
		require.NoError(t, err)
		ctx2 := context.WithValue(t.Context(), saga.CtxKey, tx2)

		batch2, err := repo.Outbox.SelectBatchForUpdate(ctx2, 2)
		require.NoError(t, err)
		require.Len(t, batch2, 2)

		ids1 := map[int64]struct{}{batch1[0].ID: {}, batch1[1].ID: {}}
		for _, e := range batch2 {
			_, overlap := ids1[e.ID]
			require.False(t, overlap, "batch2 must not contain rows locked by batch1")
		}

		require.NoError(t, tx1.Rollback(t.Context()))
		require.NoError(t, tx2.Rollback(t.Context()))
	})
}
