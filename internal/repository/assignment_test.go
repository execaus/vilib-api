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

	account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
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

	video, err := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady, false)
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

func TestRepository_AssignmentUpdateDue_SwitchesModeAndClearsOppositeField(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)
		require.NotNil(t, assignment.DueAt, "фикстура создаётся в режиме date")

		dueDays := 14

		updated, err := r.Assignment.UpdateDue(
			t.Context(), assignment.ID, domain.AssignmentDueModeDays, nil, &dueDays,
		)

		require.NoError(t, err)
		require.Equal(t, domain.AssignmentDueModeDays, updated.DueMode)
		require.Nil(t, updated.DueAt, "поле противоположного режима обнуляется")
		require.NotNil(t, updated.DueDays)
		require.Equal(t, dueDays, *updated.DueDays)
	})
}

func TestRepository_AssignmentUpdateComment_SetsAndClears(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)

		withComment, err := r.Assignment.UpdateComment(t.Context(), assignment.ID, "Пройти до конца месяца")
		require.NoError(t, err)
		require.Equal(t, "Пройти до конца месяца", withComment.Comment)

		cleared, err := r.Assignment.UpdateComment(t.Context(), assignment.ID, "")
		require.NoError(t, err)
		require.Empty(t, cleared.Comment)
	})
}

func TestRepository_AssignmentCancel_SecondCallDoesNotUpdate(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		cancelledBy := fixture.Video.Author
		now := time.Now().UTC().Truncate(time.Millisecond)

		cancelled, err := r.Assignment.Cancel(
			t.Context(), assignment.ID, &cancelledBy, domain.AssignmentCancelReasonManual, now,
		)
		require.NoError(t, err)
		require.True(t, cancelled)

		repeated, err := r.Assignment.Cancel(
			t.Context(), assignment.ID, &cancelledBy, domain.AssignmentCancelReasonManual, now,
		)
		require.NoError(t, err)
		require.False(t, repeated, "повторная отмена не обновляет строку")

		stored, err := r.Assignment.SelectByID(t.Context(), assignment.ID)
		require.NoError(t, err)
		require.Equal(t, domain.AssignmentStatusCancelled, stored.Status)
		require.NotNil(t, stored.CancelReason)
		require.Equal(t, domain.AssignmentCancelReasonManual, *stored.CancelReason)
		require.NotNil(t, stored.CancelledBy)
		require.Equal(t, cancelledBy, *stored.CancelledBy)
	})
}

func TestRepository_AssignmentCancel_SystemCancelWithoutActor(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, _ := newTestAssignment(t, r, f)

		cancelled, err := r.Assignment.Cancel(
			t.Context(), assignment.ID, nil, domain.AssignmentCancelReasonVideoDeleted,
			time.Now().UTC().Truncate(time.Millisecond),
		)

		require.NoError(t, err)
		require.True(t, cancelled)

		stored, err := r.Assignment.SelectByID(t.Context(), assignment.ID)
		require.NoError(t, err)
		require.Nil(t, stored.CancelledBy, "системная отмена выполняется без инициатора")
	})
}

func TestRepository_AssignmentSelectActiveByTargetGroup_SkipsCancelledAndForeignTargets(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		groupID := fixture.Video.GroupID

		cancelled, _ := newTestAssignment(t, r, f)
		foreignGroupID := uuid.New()

		_, err := r.AssignmentTarget.InsertBatch(t.Context(), []domain.AssignmentTarget{
			{AssignmentID: assignment.ID, TargetType: domain.AssignmentTargetTypeGroup, TargetID: groupID},
			{AssignmentID: assignment.ID, TargetType: domain.AssignmentTargetTypeUser, TargetID: uuid.New()},
			{AssignmentID: cancelled.ID, TargetType: domain.AssignmentTargetTypeGroup, TargetID: groupID},
			{AssignmentID: cancelled.ID, TargetType: domain.AssignmentTargetTypeGroup, TargetID: foreignGroupID},
		})
		require.NoError(t, err)

		_, err = r.Assignment.Cancel(
			t.Context(), cancelled.ID, nil, domain.AssignmentCancelReasonManual,
			time.Now().UTC().Truncate(time.Millisecond),
		)
		require.NoError(t, err)

		active, err := r.Assignment.SelectActiveByTargetGroup(t.Context(), groupID)

		require.NoError(t, err)
		require.Len(t, active, 1)
		require.Equal(t, assignment.ID, active[0].ID)
	})
}

func TestRepository_AssignmentSelectActiveByVideoIDs_ReturnsOnlyActive(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)

		cancelled, err := r.Assignment.Insert(
			t.Context(),
			fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)

		_, err = r.Assignment.Cancel(
			t.Context(), cancelled.ID, nil, domain.AssignmentCancelReasonVideoDeleted,
			time.Now().UTC().Truncate(time.Millisecond),
		)
		require.NoError(t, err)

		active, err := r.Assignment.SelectActiveByVideoIDs(t.Context(), []uuid.UUID{fixture.Video.ID})

		require.NoError(t, err)
		require.Len(t, active, 1)
		require.Equal(t, assignment.ID, active[0].ID)

		empty, err := r.Assignment.SelectActiveByVideoIDs(t.Context(), nil)
		require.NoError(t, err)
		require.Empty(t, empty)
	})
}

func TestRepository_AssignmentSelectActiveByGroupID_ReturnsAssignmentsOfGroupVideos(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		other, _ := newTestAssignment(t, r, f)

		found, err := r.Assignment.SelectActiveByGroupID(t.Context(), fixture.Video.GroupID)

		require.NoError(t, err)
		require.Len(t, found, 1)
		require.Equal(t, assignment.ID, found[0].ID)
		require.NotEqual(t, other.ID, found[0].ID)
	})
}

func TestRepository_AssignmentSelectByFilter_ScopeAllSeesWholeAccount(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)
		otherGroup, err := r.UserGroup.Insert(t.Context(), fixture.AccountID, f.Beer().Name())
		require.NoError(t, err)
		otherVideo, err := r.Video.Insert(
			t.Context(),
			f.Beer().Name(),
			otherGroup.ID,
			fixture.Video.Author,
			domain.VideoStatusReady,
			false,
		)
		require.NoError(t, err)

		first, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)
		second, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, otherVideo.ID, otherVideo.Name,
			otherGroup.ID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)

		got, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID,
			Scope:     repository.AssignmentScope{All: true},
		})

		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{first.ID, second.ID}, assignmentIDs(got))
	})
}

func TestRepository_AssignmentSelectByFilter_ScopeLimitsToGroupsOrOwnAssignments(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)
		otherGroup, err := r.UserGroup.Insert(t.Context(), fixture.AccountID, f.Beer().Name())
		require.NoError(t, err)
		otherVideo, err := r.Video.Insert(
			t.Context(),
			f.Beer().Name(),
			otherGroup.ID,
			fixture.Video.Author,
			domain.VideoStatusReady,
			false,
		)
		require.NoError(t, err)
		foreignCreator := newTestUser(t, r, f, fixture.AccountRoleID)

		// Назначение в области (группа видео принадлежит списку GroupIDs).
		inGroup, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)

		// Собственное назначение инициатора вне области групп — видно по created_by.
		ownedOutsideGroup, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, otherVideo.ID, otherVideo.Name,
			otherGroup.ID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)

		// Чужое назначение вне области групп — не видно.
		_, err = r.Assignment.Insert(
			t.Context(), fixture.AccountID, otherVideo.ID, otherVideo.Name,
			otherGroup.ID, f.Beer().Name(), foreignCreator.ID,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)

		got, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID,
			Scope: repository.AssignmentScope{
				All: false, GroupIDs: []uuid.UUID{fixture.Video.GroupID}, CreatedBy: fixture.Video.Author,
			},
		})

		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{inGroup.ID, ownedOutsideGroup.ID}, assignmentIDs(got))
	})
}

func TestRepository_AssignmentSelectByFilter_FiltersByGroupVideoStatusAndUser(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assignment, fixture := newTestAssignment(t, r, f)
		other, otherFixture := newTestAssignment(t, r, f)
		require.NotEqual(t, assignment.ID, other.ID)

		_, err := r.Assignment.Cancel(
			t.Context(), other.ID, nil, domain.AssignmentCancelReasonManual,
			time.Now().UTC().Truncate(time.Millisecond),
		)
		require.NoError(t, err)

		participant := newTestUser(t, r, f, fixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)
		_, err = r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: participant.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: now.Add(24 * time.Hour),
			},
		})
		require.NoError(t, err)

		scopeAll := repository.AssignmentScope{All: true}

		byGroup, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID, Scope: scopeAll, GroupID: &fixture.Video.GroupID,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{assignment.ID}, assignmentIDs(byGroup))

		byVideo, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID, Scope: scopeAll, VideoID: &fixture.Video.ID,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{assignment.ID}, assignmentIDs(byVideo))

		cancelledStatus := domain.AssignmentStatusCancelled
		byStatus, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: otherFixture.AccountID, Scope: repository.AssignmentScope{All: true}, Status: &cancelledStatus,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{other.ID}, assignmentIDs(byStatus))

		byUser, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID, Scope: scopeAll, UserID: &participant.ID,
		})
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{assignment.ID}, assignmentIDs(byUser))
	})
}

func TestRepository_AssignmentSelectByFilter_FiltersByDueRange(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)

		inRangeDueAt := time.Now().Add(5 * 24 * time.Hour).UTC().Truncate(time.Millisecond)
		outOfRangeDueAt := time.Now().Add(60 * 24 * time.Hour).UTC().Truncate(time.Millisecond)

		inRange, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDate, &inRangeDueAt, nil, "",
		)
		require.NoError(t, err)
		_, err = r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDate, &outOfRangeDueAt, nil, "",
		)
		require.NoError(t, err)
		// due_mode=days назначение без due_at — вне диапазона по построению фильтра.
		_, err = r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(5), "",
		)
		require.NoError(t, err)

		dueFrom := time.Now().UTC()
		dueTo := time.Now().Add(10 * 24 * time.Hour).UTC()

		got, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID, Scope: repository.AssignmentScope{All: true},
			DueFrom: &dueFrom, DueTo: &dueTo,
		})

		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{inRange.ID}, assignmentIDs(got))
	})
}

func TestRepository_AssignmentSelectByFilter_DueRangeMatchesPersonalDueInDaysMode(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)

		// due_mode=days: у самого назначения due_at не заполнен, срок — только персональный
		// (assignment_participants.due_at), поэтому попадание в период проверяется через
		// участника (В-61).
		assignment, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(7), "",
		)
		require.NoError(t, err)

		participant := newTestUser(t, r, f, fixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)
		personalDueAt := now.Add(5 * 24 * time.Hour)
		_, err = r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: participant.ID,
				Status: domain.AssignmentParticipantStatusAssigned, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: personalDueAt,
			},
		})
		require.NoError(t, err)

		dueFrom := now
		dueTo := now.Add(10 * 24 * time.Hour)

		got, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID, Scope: repository.AssignmentScope{All: true},
			DueFrom: &dueFrom, DueTo: &dueTo,
		})

		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{assignment.ID}, assignmentIDs(got))
	})
}

func TestRepository_AssignmentSelectByFilter_DueRangeExcludesAssignmentWhereAllParticipantsCancelled(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		fixture := newTestAccountAndVideo(t, r, f)

		assignment, err := r.Assignment.Insert(
			t.Context(), fixture.AccountID, fixture.Video.ID, fixture.Video.Name,
			fixture.Video.GroupID, f.Beer().Name(), fixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(7), "",
		)
		require.NoError(t, err)

		participant := newTestUser(t, r, f, fixture.AccountRoleID)
		now := time.Now().UTC().Truncate(time.Millisecond)
		personalDueAt := now.Add(5 * 24 * time.Hour)
		// Персональный срок формально попадает в период, но участие отменено — обязанности
		// пройти видео в этом периоде уже нет, назначение не должно попасть в выборку.
		_, err = r.AssignmentParticipant.InsertBatch(t.Context(), []domain.AssignmentParticipant{
			{
				AssignmentID: assignment.ID, UserID: participant.ID,
				Status: domain.AssignmentParticipantStatusCancelled, Source: domain.AssignmentParticipantSourcePersonal,
				EnrolledAt: now, DueAt: personalDueAt,
			},
		})
		require.NoError(t, err)

		dueFrom := now
		dueTo := now.Add(10 * 24 * time.Hour)

		got, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: fixture.AccountID, Scope: repository.AssignmentScope{All: true},
			DueFrom: &dueFrom, DueTo: &dueTo,
		})

		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// assignmentIDs собирает идентификаторы назначений для сравнения без учёта порядка —
// SelectByFilter не гарантирует сортировку (сортировка на клиенте, §5 дизайна эпика Э3).
func assignmentIDs(assignments []domain.Assignment) []uuid.UUID {
	ids := make([]uuid.UUID, len(assignments))
	for i, a := range assignments {
		ids[i] = a.ID
	}

	return ids
}

// ptrInt — указатель на литерал int для необязательных полей репозитория.
func ptrInt(v int) *int { return &v }

// TestRepository_AssignmentSelectByFilter_DueRangeKeepsAccountIsolation проверяет, что
// дизъюнкция внутри условия периода (В-61) не ломает приоритет операторов в WHERE: назначение
// чужого аккаунта, персональный срок участника которого попадает в период, в выборку не
// попадает.
func TestRepository_AssignmentSelectByFilter_DueRangeKeepsAccountIsolation(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		own, ownFixture := newTestAssignment(t, r, f)
		foreign, foreignFixture := newTestAssignment(t, r, f)

		now := time.Now().UTC().Truncate(time.Millisecond)
		dueAt := now.Add(48 * time.Hour)

		// У обоих назначений есть участник с персональным сроком внутри периода.
		insertTestParticipant(
			t, r, own.ID, newTestUser(t, r, f, ownFixture.AccountRoleID).ID,
			domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourcePersonal, nil, now, dueAt,
		)
		insertTestParticipant(
			t, r, foreign.ID, newTestUser(t, r, f, foreignFixture.AccountRoleID).ID,
			domain.AssignmentParticipantStatusAssigned,
			domain.AssignmentParticipantSourcePersonal, nil, now, dueAt,
		)

		from := now
		to := now.Add(72 * time.Hour)

		found, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: ownFixture.AccountID,
			Scope:     repository.AssignmentScope{All: true},
			DueFrom:   &from,
			DueTo:     &to,
		})

		require.NoError(t, err)
		for _, a := range found {
			require.Equal(t, ownFixture.AccountID, a.AccountID,
				"фильтр периода не должен выводить назначения чужого аккаунта")
		}
	})
}

// TestRepository_AssignmentSelectByFilter_ScopeKeepsAccountIsolation проверяет ту же защиту
// приоритета операторов для области видимости В-8: дизъюнкция «группа инициатора ИЛИ его
// собственные назначения» не должна выпускать назначения чужого аккаунта, созданные тем же
// пользователем.
func TestRepository_AssignmentSelectByFilter_ScopeKeepsAccountIsolation(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		own, ownFixture := newTestAssignment(t, r, f)
		foreignFixture := newTestAccountAndVideo(t, r, f)

		// Назначение чужого аккаунта, созданное тем же пользователем, что и своё.
		foreign, err := r.Assignment.Insert(
			t.Context(),
			foreignFixture.AccountID, foreignFixture.Video.ID, foreignFixture.Video.Name,
			foreignFixture.Video.GroupID, f.Beer().Name(), ownFixture.Video.Author,
			domain.AssignmentDueModeDays, nil, ptrInt(7), "",
		)
		require.NoError(t, err)

		found, err := r.Assignment.SelectByFilter(t.Context(), repository.AssignmentFilter{
			AccountID: ownFixture.AccountID,
			Scope: repository.AssignmentScope{
				GroupIDs:  []uuid.UUID{ownFixture.Video.GroupID, foreignFixture.Video.GroupID},
				CreatedBy: ownFixture.Video.Author,
			},
		})

		require.NoError(t, err)
		ids := make([]uuid.UUID, len(found))
		for i, a := range found {
			require.Equal(t, ownFixture.AccountID, a.AccountID,
				"область видимости не должна выпускать назначения чужого аккаунта")
			ids[i] = a.ID
		}
		require.Contains(t, ids, own.ID)
		require.NotContains(t, ids, foreign.ID)
	})
}
