package repository_test

import (
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

// TestRepository_PipelineProgressSelect_ReturnsSeedRowsPerLane проверяет, что миграция заводит
// ровно две строки индикатора прогресса (архивную и срочную полосы, эпик Э5, исправление Д-1) —
// и что Select читает каждую по своему ключу.
func TestRepository_PipelineProgressSelect_ReturnsSeedRowsPerLane(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		archival, err := r.PipelineProgress.Select(t.Context(), false)
		require.NoError(t, err)
		require.False(t, archival.IsUrgent)
		require.WithinDuration(t, time.Now(), archival.LastDequeuedAt, time.Minute)

		urgent, err := r.PipelineProgress.Select(t.Context(), true)
		require.NoError(t, err)
		require.True(t, urgent.IsUrgent)
		require.WithinDuration(t, time.Now(), urgent.LastDequeuedAt, time.Minute)
	})
}

// TestRepository_PipelineProgressUpdateLastDequeuedAt_UpdatesOnlyTargetLane проверяет, что
// UpdateLastDequeuedAt меняет last_dequeued_at ровно указанной полосы, не затрагивая другую
// (эпик Э5, исправление Д-1: раздельные строки на полосу).
func TestRepository_PipelineProgressUpdateLastDequeuedAt_UpdatesOnlyTargetLane(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		before, err := r.PipelineProgress.Select(t.Context(), false)
		require.NoError(t, err)

		archivalBefore, err := r.PipelineProgress.Select(t.Context(), true)
		require.NoError(t, err)

		now := before.LastDequeuedAt.Add(time.Hour)
		require.NoError(t, r.PipelineProgress.UpdateLastDequeuedAt(t.Context(), false, now))

		archival, err := r.PipelineProgress.Select(t.Context(), false)
		require.NoError(t, err)
		require.WithinDuration(t, now, archival.LastDequeuedAt, time.Second)

		urgent, err := r.PipelineProgress.Select(t.Context(), true)
		require.NoError(t, err)
		require.WithinDuration(t, archivalBefore.LastDequeuedAt, urgent.LastDequeuedAt, time.Second)
		require.Equal(t, domain.PipelineProgress{IsUrgent: true, LastDequeuedAt: urgent.LastDequeuedAt}, urgent)
	})
}
