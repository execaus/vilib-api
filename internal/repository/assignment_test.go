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

// assignmentFixture — окружение, достаточное для тестов репозиториев назначения: аккаунт,
// роль аккаунта (для создания дополнительных участников), группа и видео.
type assignmentFixture struct {
	AccountID     uuid.UUID
	AccountRoleID uuid.UUID
	Video         domain.Video
}

// newTestAssignment создаёт аккаунт, группу, пользователя, видео и назначение с фиксированным
// сроком (due_mode=date) — минимальный набор данных для тестов репозиториев назначения.
// assignments.account_id/video_id/group_id — внешние ключи, поэтому нельзя обойтись
// сгенерированными uuid.New(), как для полиморфных ссылок в assignment_targets.
func newTestAssignment(
	t *testing.T, r *repository.Repository, f faker.Faker,
) (domain.Assignment, assignmentFixture) {
	t.Helper()

	fixture := newTestAccountAndVideo(t, r, f)
	dueAt := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Millisecond)

	assignment, err := r.Assignment.Insert(
		t.Context(),
		fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
		fixture.Video.GroupID, f.Beer().Name(),
		fixture.Video.Author,
		domain.AssignmentDueModeDate, &dueAt, nil,
		"",
	)
	require.NoError(t, err)

	return assignment, fixture
}

// newTestAccountAndVideo создаёт аккаунт, группу, роль, пользователя и видео — вариант
// newTestVideo, дополнительно возвращающий accountID/accountRoleID для внешних ключей
// assignments/assignment_participants.
func newTestAccountAndVideo(t *testing.T, r *repository.Repository, f faker.Faker) assignmentFixture {
	t.Helper()

	account, err := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
	require.NoError(t, err)

	group, err := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
	require.NoError(t, err)

	accountRole, err := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
	require.NoError(t, err)

	user, err := r.User.Insert(
		t.Context(), f.Person().FirstName(), f.Person().LastName(), f.Hash().MD5(),
		f.Person().Contact().Email, accountRole.ID,
	)
	require.NoError(t, err)

	video, err := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady)
	require.NoError(t, err)

	return assignmentFixture{AccountID: account.ID, AccountRoleID: accountRole.ID, Video: video}
}

// newTestUser создаёт пользователя в роли roleID — участник назначения для тестов
// assignment_participants (user_id — внешний ключ на users).
func newTestUser(t *testing.T, r *repository.Repository, f faker.Faker, roleID uuid.UUID) domain.User {
	t.Helper()

	user, err := r.User.Insert(
		t.Context(), f.Person().FirstName(), f.Person().LastName(), f.Hash().MD5(),
		f.Person().Contact().Email, roleID,
	)
	require.NoError(t, err)

	return user
}

func TestRepository_AssignmentInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)
		video := fixture.Video
		dueAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Millisecond)

		assignment, err := r.Assignment.Insert(
			t.Context(),
			fixture.AccountID, video.ID, video.Name,
			video.GroupID, "Продажи",
			video.Author,
			domain.AssignmentDueModeDate, &dueAt, nil,
			"комментарий",
		)

		require.NoError(t, err)
		require.NotEmpty(t, assignment.ID)
		require.Equal(t, fixture.AccountID, assignment.AccountID)
		require.Equal(t, video.ID, *assignment.VideoID)
		require.Equal(t, video.Name, assignment.VideoName)
		require.Equal(t, video.GroupID, *assignment.GroupID)
		require.Equal(t, "Продажи", assignment.GroupName)
		require.Equal(t, video.Author, assignment.CreatedBy)
		require.Equal(t, domain.AssignmentDueModeDate, assignment.DueMode)
		require.WithinDuration(t, dueAt, *assignment.DueAt, time.Millisecond)
		require.Nil(t, assignment.DueDays)
		require.Equal(t, "комментарий", assignment.Comment)
		require.Equal(t, domain.AssignmentStatusActive, assignment.Status)
		require.Nil(t, assignment.CancelledAt)
	})
}

func TestRepository_AssignmentInsert_DueDaysMode(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)
		video := fixture.Video
		dueDays := 7

		assignment, err := r.Assignment.Insert(
			t.Context(),
			fixture.AccountID, video.ID, video.Name,
			video.GroupID, f.Beer().Name(),
			video.Author,
			domain.AssignmentDueModeDays, nil, &dueDays,
			"",
		)

		require.NoError(t, err)
		require.Equal(t, domain.AssignmentDueModeDays, assignment.DueMode)
		require.Nil(t, assignment.DueAt)
		require.Equal(t, dueDays, *assignment.DueDays)
	})
}

func TestRepository_AssignmentSelectByID_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		_, err := r.Assignment.SelectByID(t.Context(), uuid.New())

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_AssignmentSelectByID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		created, _ := newTestAssignment(t, r, f)

		got, err := r.Assignment.SelectByID(t.Context(), created.ID)

		require.NoError(t, err)
		require.Equal(t, created.ID, got.ID)
		require.Equal(t, created.VideoName, got.VideoName)
	})
}

func TestRepository_AssignmentSelectByIDs_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		first, _ := newTestAssignment(t, r, f)
		second, _ := newTestAssignment(t, r, f)

		got, err := r.Assignment.SelectByIDs(t.Context(), []uuid.UUID{first.ID, second.ID, uuid.New()})

		require.NoError(t, err)
		require.Len(t, got, 2)
	})
}

func TestRepository_AssignmentSelectByIDs_EmptyInputNoQuery(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		got, err := r.Assignment.SelectByIDs(t.Context(), nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}
