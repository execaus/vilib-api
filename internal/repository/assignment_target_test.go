package repository_test

import (
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRepository_AssignmentTargetInsertBatch_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		userTargetID := uuid.New()

		targets, err := r.AssignmentTarget.InsertBatch(t.Context(), []domain.AssignmentTarget{
			{
				AssignmentID: assignment.ID,
				TargetType:   domain.AssignmentTargetTypeGroup,
				TargetID:     fixture.Video.GroupID,
			},
			{AssignmentID: assignment.ID, TargetType: domain.AssignmentTargetTypeUser, TargetID: userTargetID},
		})

		require.NoError(t, err)
		require.Len(t, targets, 2)
	})
}

func TestRepository_AssignmentTargetSelectByAssignmentIDs_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		first, firstFixture := newTestAssignment(t, r, f)
		second, _ := newTestAssignment(t, r, f)

		_, err := r.AssignmentTarget.InsertBatch(t.Context(), []domain.AssignmentTarget{
			{
				AssignmentID: first.ID,
				TargetType:   domain.AssignmentTargetTypeGroup,
				TargetID:     firstFixture.Video.GroupID,
			},
		})
		require.NoError(t, err)

		got, err := r.AssignmentTarget.SelectByAssignmentIDs(t.Context(), []uuid.UUID{first.ID, second.ID})

		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, first.ID, got[0].AssignmentID)
		require.Equal(t, domain.AssignmentTargetTypeGroup, got[0].TargetType)
		require.Equal(t, firstFixture.Video.GroupID, got[0].TargetID)
	})
}

func TestRepository_AssignmentTargetSelectByAssignmentIDs_EmptyInputNoQuery(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		got, err := r.AssignmentTarget.SelectByAssignmentIDs(t.Context(), nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}
