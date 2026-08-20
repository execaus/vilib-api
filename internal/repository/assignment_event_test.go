package repository_test

import (
	"encoding/json"
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRepository_AssignmentEventInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)
		actorID := assignment.CreatedBy
		now := time.Now().UTC().Truncate(time.Millisecond)
		payload := json.RawMessage(`{"reason":"manual"}`)

		event, err := r.AssignmentEvent.Insert(
			t.Context(), assignment.ID, nil, domain.AssignmentEventTypeCreated, &actorID, payload, now,
		)

		require.NoError(t, err)
		require.NotEmpty(t, event.ID)
		require.Equal(t, assignment.ID, event.AssignmentID)
		require.Nil(t, event.UserID)
		require.Equal(t, domain.AssignmentEventTypeCreated, event.Type)
		require.Equal(t, actorID, *event.ActorID)
		require.JSONEq(t, string(payload), string(event.Payload))
	})
}

func TestRepository_AssignmentEventInsertBatch_PreservesOrder(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)
		userID := uuid.New()
		now := time.Now().UTC().Truncate(time.Millisecond)

		inserted, err := r.AssignmentEvent.InsertBatch(t.Context(), []domain.AssignmentEvent{
			{
				AssignmentID: assignment.ID, Type: domain.AssignmentEventTypeCreated,
				CreatedAt: now, Payload: json.RawMessage(`{}`),
			},
			{
				AssignmentID: assignment.ID, UserID: &userID, Type: domain.AssignmentEventTypeParticipantEnrolled,
				CreatedAt: now.Add(time.Second), Payload: json.RawMessage(`{"source":"personal"}`),
			},
		})

		require.NoError(t, err)
		require.Len(t, inserted, 2)

		events, err := r.AssignmentEvent.SelectByAssignmentID(t.Context(), assignment.ID)
		require.NoError(t, err)
		require.Len(t, events, 2)
		require.Equal(t, domain.AssignmentEventTypeCreated, events[0].Type)
		require.Equal(t, domain.AssignmentEventTypeParticipantEnrolled, events[1].Type)
		require.Equal(t, userID, *events[1].UserID)
		require.Less(t, events[0].ID, events[1].ID)
	})
}

func TestRepository_AssignmentEventSelectByAssignmentID_EmptyWhenNoEvents(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)

		got, err := r.AssignmentEvent.SelectByAssignmentID(t.Context(), assignment.ID)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}
