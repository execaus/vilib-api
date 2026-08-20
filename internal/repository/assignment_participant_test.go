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

		counters, err := r.AssignmentParticipant.CountByAssignmentIDs(t.Context(), []uuid.UUID{assignment.ID}, true)

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

func TestRepository_AssignmentParticipantCountByAssignmentIDs_ExcludesDeactivatedWhenNotIncluded(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		now := time.Now().UTC().Truncate(time.Millisecond)

		activeUser := newTestUser(t, r, f, fixture.AccountRoleID)
		deactivatedUser := newTestUser(t, r, f, fixture.AccountRoleID)

		_, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: activeUser.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(-time.Hour), // просрочен
			},
			{
				AssignmentID: assignment.ID, UserID: deactivatedUser.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(-time.Hour), // тоже просрочен, но деактивирован
			},
		})
		require.NoError(t, err)

		require.NoError(t, r.User.Deactivate(t.Context(), deactivatedUser.ID))

		withoutDeactivated, err := r.AssignmentParticipant.CountByAssignmentIDs(
			t.Context(),
			[]uuid.UUID{assignment.ID},
			false,
		)
		require.NoError(t, err)
		require.Equal(t, 1, withoutDeactivated[assignment.ID].Total)
		require.Equal(t, 1, withoutDeactivated[assignment.ID].Overdue)

		withDeactivated, err := r.AssignmentParticipant.CountByAssignmentIDs(
			t.Context(),
			[]uuid.UUID{assignment.ID},
			true,
		)
		require.NoError(t, err)
		require.Equal(t, 2, withDeactivated[assignment.ID].Total)
		require.Equal(t, 2, withDeactivated[assignment.ID].Overdue)
	})
}

func TestRepository_AssignmentParticipantCountByAssignmentIDs_EmptyInputNoQuery(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		got, err := r.AssignmentParticipant.CountByAssignmentIDs(t.Context(), nil, true)

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

// insertTestParticipant создаёт персональную запись участника назначения в заданном статусе —
// исходное состояние для тестов пересчёта срока и отмены.
func insertTestParticipant(
	t *testing.T, r *repository.Repository,
	assignmentID, userID uuid.UUID,
	status domain.AssignmentParticipantStatus,
	source domain.AssignmentParticipantSource, sourceGroupID *uuid.UUID,
	enrolledAt, dueAt time.Time,
) {
	t.Helper()

	inserted, err := r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
		{
			AssignmentID:  assignmentID,
			UserID:        userID,
			Status:        status,
			Source:        source,
			SourceGroupID: sourceGroupID,
			EnrolledAt:    enrolledAt,
			DueAt:         dueAt,
		},
	})
	require.NoError(t, err)
	require.Len(t, inserted, 1)
}

// participantsByUser раскладывает участников назначения по идентификатору пользователя.
func participantsByUser(
	t *testing.T, r *repository.Repository, assignmentID uuid.UUID,
) map[uuid.UUID]domain.AssignmentParticipant {
	t.Helper()

	stored, err := r.AssignmentParticipant.SelectByAssignmentIDs(t.Context(), []uuid.UUID{assignmentID})
	require.NoError(t, err)

	byUser := make(map[uuid.UUID]domain.AssignmentParticipant, len(stored))
	for _, p := range stored {
		byUser[p.UserID] = p
	}

	return byUser
}

func TestRepository_AssignmentParticipantSelectByAssignmentIDAndUserID_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)

		_, err := r.AssignmentParticipant.SelectByAssignmentIDAndUserID(t.Context(), assignment.ID, uuid.New())

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_AssignmentParticipantUpdateDueByAssignment_DateKeepsCompleted(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		now := time.Now().UTC().Truncate(time.Millisecond)
		oldDueAt := now.Add(24 * time.Hour)

		activeUser := newTestUser(t, r, f, fixture.AccountRoleID)
		completedUser := newTestUser(t, r, f, fixture.AccountRoleID)
		insertTestParticipant(
			t, r, assignment.ID, activeUser.ID, domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourcePersonal, nil, now, oldDueAt,
		)
		insertTestParticipant(
			t, r, assignment.ID, completedUser.ID, domain.AssignmentParticipantStatusCompleted,
			domain.AssignmentParticipantSourcePersonal, nil, now, oldDueAt,
		)

		newDueAt := now.Add(10 * 24 * time.Hour)

		updated, err := r.AssignmentParticipant.UpdateDueByAssignment(
			t.Context(), assignment.ID, domain.AssignmentDueModeDate, &newDueAt, nil,
		)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{activeUser.ID}, updated)

		byUser := participantsByUser(t, r, assignment.ID)
		require.WithinDuration(t, newDueAt, byUser[activeUser.ID].DueAt, time.Second)
		require.WithinDuration(t, oldDueAt, byUser[completedUser.ID].DueAt, time.Second,
			"срок завершённого участника не пересчитывается")
	})
}

func TestRepository_AssignmentParticipantUpdateDueByAssignment_DaysCountsFromEnrolledAt(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		now := time.Now().UTC().Truncate(time.Millisecond)
		enrolledAt := now.Add(-48 * time.Hour)

		user := newTestUser(t, r, f, fixture.AccountRoleID)
		insertTestParticipant(
			t, r, assignment.ID, user.ID, domain.AssignmentParticipantStatusInProgress,
			domain.AssignmentParticipantSourceGroup, &fixture.Video.GroupID, enrolledAt, now,
		)

		dueDays := 5

		updated, err := r.AssignmentParticipant.UpdateDueByAssignment(
			t.Context(), assignment.ID, domain.AssignmentDueModeDays, nil, &dueDays,
		)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{user.ID}, updated)

		byUser := participantsByUser(t, r, assignment.ID)
		require.WithinDuration(
			t, enrolledAt.Add(time.Duration(dueDays)*24*time.Hour), byUser[user.ID].DueAt, time.Second,
		)
	})
}

func TestRepository_AssignmentParticipantUpdateDueByAssignment_RejectsModeWithoutValue(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)

		_, err := r.AssignmentParticipant.UpdateDueByAssignment(
			t.Context(), assignment.ID, domain.AssignmentDueModeDate, nil, nil,
		)

		require.ErrorIs(t, err, repository.ErrInvalidDueMode)
	})
}

func TestRepository_AssignmentParticipantCancelByAssignment_CancelsOnlyUnfinished(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		now := time.Now().UTC().Truncate(time.Millisecond)

		assignedUser := newTestUser(t, r, f, fixture.AccountRoleID)
		completedUser := newTestUser(t, r, f, fixture.AccountRoleID)
		insertTestParticipant(
			t, r, assignment.ID, assignedUser.ID, domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourcePersonal, nil, now, now.Add(24*time.Hour),
		)
		insertTestParticipant(
			t, r, assignment.ID, completedUser.ID, domain.AssignmentParticipantStatusCompleted,
			domain.AssignmentParticipantSourcePersonal, nil, now, now.Add(24*time.Hour),
		)

		cancelled, err := r.AssignmentParticipant.CancelByAssignment(
			t.Context(), assignment.ID, domain.AssignmentParticipantCancelReasonAssignmentCancelled, now,
		)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{assignedUser.ID}, cancelled)

		byUser := participantsByUser(t, r, assignment.ID)
		require.Equal(t, domain.AssignmentParticipantStatusCancelled, byUser[assignedUser.ID].Status)
		require.NotNil(t, byUser[assignedUser.ID].CancelReason)
		require.Equal(
			t, domain.AssignmentParticipantCancelReasonAssignmentCancelled,
			*byUser[assignedUser.ID].CancelReason,
		)
		require.Equal(t, domain.AssignmentParticipantStatusCompleted, byUser[completedUser.ID].Status)
	})
}

func TestRepository_AssignmentParticipantCancelOne_SkipsCompleted(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		now := time.Now().UTC().Truncate(time.Millisecond)

		activeUser := newTestUser(t, r, f, fixture.AccountRoleID)
		completedUser := newTestUser(t, r, f, fixture.AccountRoleID)
		insertTestParticipant(
			t, r, assignment.ID, activeUser.ID, domain.AssignmentParticipantStatusInProgress,
			domain.AssignmentParticipantSourcePersonal, nil, now, now.Add(24*time.Hour),
		)
		insertTestParticipant(
			t, r, assignment.ID, completedUser.ID, domain.AssignmentParticipantStatusCompleted,
			domain.AssignmentParticipantSourcePersonal, nil, now, now.Add(24*time.Hour),
		)

		cancelled, err := r.AssignmentParticipant.CancelOne(
			t.Context(), assignment.ID, activeUser.ID,
			domain.AssignmentParticipantCancelReasonRemovedByManager, now,
		)
		require.NoError(t, err)
		require.True(t, cancelled)

		skipped, err := r.AssignmentParticipant.CancelOne(
			t.Context(), assignment.ID, completedUser.ID,
			domain.AssignmentParticipantCancelReasonRemovedByManager, now,
		)
		require.NoError(t, err)
		require.False(t, skipped, "завершённый участник не отменяется")

		byUser := participantsByUser(t, r, assignment.ID)
		require.Equal(t, domain.AssignmentParticipantStatusCancelled, byUser[activeUser.ID].Status)
		require.Equal(t, domain.AssignmentParticipantStatusCompleted, byUser[completedUser.ID].Status)
	})
}

func TestRepository_AssignmentParticipantCancelBySourceGroupAndUser_KeepsPersonal(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		groupAssignment, fixture := newTestAssignment(t, r, f)
		groupID := fixture.Video.GroupID
		now := time.Now().UTC().Truncate(time.Millisecond)

		personalAssignment, err := r.Assignment.Insert(
			t.Context(),
			fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			groupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(3), "",
		)
		require.NoError(t, err)

		user := newTestUser(t, r, f, fixture.AccountRoleID)
		insertTestParticipant(
			t, r, groupAssignment.ID, user.ID, domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourceGroup, &groupID, now, now.Add(24*time.Hour),
		)
		insertTestParticipant(
			t, r, personalAssignment.ID, user.ID, domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourcePersonal, nil, now, now.Add(24*time.Hour),
		)

		cancelled, err := r.AssignmentParticipant.CancelBySourceGroupAndUser(
			t.Context(), groupID, user.ID, domain.AssignmentParticipantCancelReasonLeftGroup, now,
		)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{groupAssignment.ID}, cancelled)

		byGroupAssignment := participantsByUser(t, r, groupAssignment.ID)
		require.Equal(t, domain.AssignmentParticipantStatusCancelled, byGroupAssignment[user.ID].Status)
		require.NotNil(t, byGroupAssignment[user.ID].CancelReason)
		require.Equal(
			t, domain.AssignmentParticipantCancelReasonLeftGroup, *byGroupAssignment[user.ID].CancelReason,
		)

		byPersonalAssignment := participantsByUser(t, r, personalAssignment.ID)
		require.Equal(t, domain.AssignmentParticipantStatusAssigned, byPersonalAssignment[user.ID].Status,
			"личное назначение исключением из группы не затрагивается")
	})
}

func TestRepository_AssignmentParticipantCancelBySourceGroupAndUser_SkipsCancelledAssignment(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		groupID := fixture.Video.GroupID
		now := time.Now().UTC().Truncate(time.Millisecond)

		user := newTestUser(t, r, f, fixture.AccountRoleID)
		insertTestParticipant(
			t, r, assignment.ID, user.ID, domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourceGroup, &groupID, now, now.Add(24*time.Hour),
		)

		_, err := r.Assignment.Cancel(
			t.Context(), assignment.ID, nil, domain.AssignmentCancelReasonManual, now,
		)
		require.NoError(t, err)

		cancelled, err := r.AssignmentParticipant.CancelBySourceGroupAndUser(
			t.Context(), groupID, user.ID, domain.AssignmentParticipantCancelReasonLeftGroup, now,
		)

		require.NoError(t, err)
		require.Empty(t, cancelled, "участники отменённого назначения не трогаются")
	})
}
