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

func TestRepository_AssignmentParticipantInsertBatch_InsertsNewRows(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		user := newTestUser(t, r, f, fixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)

		inserted, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: user.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
		})

		require.NoError(t, err)
		require.Len(t, inserted, 1)
		require.Equal(t, domain.AssignmentParticipantStatusAssigned, inserted[0].Status)
		require.Nil(t, inserted[0].CancelledAt)
	})
}

func TestRepository_AssignmentParticipantInsertBatch_ReactivatesCancelledAndSkipsCompleted(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		cancelledUser := newTestUser(t, r, f, fixture.AccountRoleID)
		completedUser := newTestUser(t, r, f, fixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)

		// Исходное состояние: один отменённый и один завершённый участник.
		_, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: cancelledUser.ID,
				Status: domain.AssignmentParticipantStatusCancelled, Source: domain.AssignmentParticipantSourceGroup,
				EnrolledAt: now.Add(-48 * time.Hour), DueAt: now.Add(-24 * time.Hour),
			},
			{
				AssignmentID: assignment.ID, UserID: completedUser.ID,
				Status: domain.AssignmentParticipantStatusCompleted, Source: domain.AssignmentParticipantSourceGroup,
				EnrolledAt: now.Add(-48 * time.Hour), DueAt: now.Add(-24 * time.Hour),
			},
		})
		require.NoError(t, err)

		newUser := newTestUser(t, r, f, fixture.AccountRoleID)
		newEnrolledAt := now
		newDueAt := now.Add(7 * 24 * time.Hour)

		// Повторная попытка зачисления тех же двух пользователей + новый пользователь —
		// как при повторном добавлении в группу с активным назначением (OnMembersAdded, В-52).
		reactivated, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: cancelledUser.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourceGroup,
				EnrolledAt: newEnrolledAt, DueAt: newDueAt,
			},
			{
				AssignmentID: assignment.ID, UserID: completedUser.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourceGroup,
				EnrolledAt: newEnrolledAt, DueAt: newDueAt,
			},
			{
				AssignmentID: assignment.ID, UserID: newUser.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourceGroup,
				EnrolledAt: newEnrolledAt, DueAt: newDueAt,
			},
		})

		require.NoError(t, err)
		require.Len(t, reactivated, 2, "завершённый участник не должен попасть в результат")

		reactivatedByUser := make(map[uuid.UUID]domain.AssignmentParticipant, len(reactivated))
		for _, p := range reactivated {
			reactivatedByUser[p.UserID] = p
		}

		_, completedTouched := reactivatedByUser[completedUser.ID]
		require.False(t, completedTouched)

		cancelled, ok := reactivatedByUser[cancelledUser.ID]
		require.True(t, ok)
		require.Equal(t, domain.AssignmentParticipantStatusAssigned, cancelled.Status)
		require.Nil(t, cancelled.CancelledAt)
		require.Nil(t, cancelled.CancelReason)
		require.WithinDuration(t, newEnrolledAt, cancelled.EnrolledAt, time.Millisecond)

		newRow, ok := reactivatedByUser[newUser.ID]
		require.True(t, ok)
		require.Equal(t, domain.AssignmentParticipantStatusAssigned, newRow.Status)

		// Завершённый участник в БД не изменился.
		all, err := r.AssignmentParticipant.SelectByAssignmentIDs(t.Context(), []uuid.UUID{assignment.ID})
		require.NoError(t, err)

		byUser := make(map[uuid.UUID]domain.AssignmentParticipant, len(all))
		for _, p := range all {
			byUser[p.UserID] = p
		}
		require.Equal(t, domain.AssignmentParticipantStatusCompleted, byUser[completedUser.ID].Status)
		require.WithinDuration(t, now.Add(-48*time.Hour), byUser[completedUser.ID].EnrolledAt, time.Millisecond)
	})
}

func TestRepository_AssignmentParticipantSelectByAssignmentIDs_EmptyInputNoQuery(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		got, err := r.AssignmentParticipant.SelectByAssignmentIDs(t.Context(), nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestRepository_AssignmentParticipantSelectByUserID_ReturnsAllStatuses(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		first, fixture := newTestAssignment(t, r, f)
		second, secondFixture := newTestAssignment(t, r, f)
		user := newTestUser(t, r, f, fixture.AccountRoleID)
		otherUser := newTestUser(t, r, f, secondFixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)

		_, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: first.ID, UserID: user.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
			{
				AssignmentID: second.ID, UserID: user.ID,
				Status: domain.AssignmentParticipantStatusCancelled, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
			{
				AssignmentID: second.ID, UserID: otherUser.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
		})
		require.NoError(t, err)

		got, err := r.AssignmentParticipant.SelectByUserID(t.Context(), user.ID)

		require.NoError(t, err)
		require.Len(t, got, 2)
	})
}

func TestRepository_AssignmentParticipantCountByAssignmentIDs_AggregatesStatusesAndOverdue(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		now := time.Now().UTC().Truncate(time.Millisecond)

		_, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID,
				UserID:       newTestUser(t, r, f, fixture.AccountRoleID).ID,
				Status:       domain.AssignmentParticipantStatusAssigned,
				Source:       domain.AssignmentParticipantSourcePersonal,
				EnrolledAt:   now,
				DueAt:        now.Add(-time.Hour), // просрочен
			},
			{
				AssignmentID: assignment.ID,
				UserID:       newTestUser(t, r, f, fixture.AccountRoleID).ID,
				Status:       domain.AssignmentParticipantStatusInProgress,
				Source:       domain.AssignmentParticipantSourcePersonal,
				EnrolledAt:   now,
				DueAt:        now.Add(24 * time.Hour),
			},
			{
				AssignmentID: assignment.ID,
				UserID:       newTestUser(t, r, f, fixture.AccountRoleID).ID,
				Status:       domain.AssignmentParticipantStatusCompleted,
				Source:       domain.AssignmentParticipantSourcePersonal,
				EnrolledAt:   now,
				// просрочен, но выполнен — не считается overdue
				DueAt: now.Add(-time.Hour),
			},
			{
				AssignmentID: assignment.ID,
				UserID:       newTestUser(t, r, f, fixture.AccountRoleID).ID,
				Status:       domain.AssignmentParticipantStatusCancelled,
				Source:       domain.AssignmentParticipantSourcePersonal,
				EnrolledAt:   now,
				DueAt:        now.Add(24 * time.Hour),
			},
		})
		require.NoError(t, err)

		counters, err := r.AssignmentParticipant.CountByAssignmentIDs(t.Context(), []uuid.UUID{assignment.ID})

		require.NoError(t, err)
		got := counters[assignment.ID]
		require.Equal(t, 4, got.Total)
		require.Equal(t, 1, got.Assigned)
		require.Equal(t, 1, got.InProgress)
		require.Equal(t, 1, got.Completed)
		require.Equal(t, 1, got.Cancelled)
		require.Equal(t, 1, got.Overdue)
	})
}

func TestRepository_AssignmentParticipantCountByAssignmentIDs_EmptyInputNoQuery(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		got, err := r.AssignmentParticipant.CountByAssignmentIDs(t.Context(), nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestRepository_AssignmentParticipantUpdateStatusByUserVideo_TransitionsAssignedToInProgress(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		user := newTestUser(t, r, f, fixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)

		_, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: user.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
		})
		require.NoError(t, err)

		ids, err := r.AssignmentParticipant.UpdateStatusByUserVideo(
			t.Context(), user.ID, fixture.Video.ID,
			domain.AssignmentParticipantStatusAssigned, domain.AssignmentParticipantStatusInProgress,
		)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{assignment.ID}, ids)

		got, err := r.AssignmentParticipant.SelectByAssignmentIDs(t.Context(), []uuid.UUID{assignment.ID})
		require.NoError(t, err)
		require.Equal(t, domain.AssignmentParticipantStatusInProgress, got[0].Status)
	})
}

func TestRepository_AssignmentParticipantCompleteByUserVideo_SetsCompletedFieldsOnce(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		user := newTestUser(t, r, f, fixture.AccountRoleID)
		sessionID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)

		_, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: user.ID,
				Status:     domain.AssignmentParticipantStatusInProgress,
				Source:     domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
		})
		require.NoError(t, err)

		ids, err := r.AssignmentParticipant.CompleteByUserVideo(
			t.Context(), user.ID, fixture.Video.ID, now, 97, 95, &sessionID,
		)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{assignment.ID}, ids)

		// Повторный вызов не переписывает уже завершённого участника (Э3-Н1).
		second, err := r.AssignmentParticipant.CompleteByUserVideo(
			t.Context(), user.ID, fixture.Video.ID, now.Add(time.Hour), 100, 95, &sessionID,
		)
		require.NoError(t, err)
		require.Empty(t, second)

		got, err := r.AssignmentParticipant.SelectByAssignmentIDs(t.Context(), []uuid.UUID{assignment.ID})
		require.NoError(t, err)
		require.Equal(t, domain.AssignmentParticipantStatusCompleted, got[0].Status)
		require.Equal(t, 97, *got[0].CompletedCoveragePct)
		require.Equal(t, sessionID, *got[0].CompletedSessionID)
	})
}
