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

func TestRepository_GroupMemberInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			userCount = 8
			name      = f.Beer().Name()
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, name)
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)

		generatedUsersID := make([]uuid.UUID, userCount)
		for i := range userCount {
			user, err := r.User.Insert(
				t.Context(),
				f.Person().FirstName(),
				f.Person().LastName(),
				f.Hash().MD5(),
				f.Person().Contact().Email,
				accountRole.ID,
			)
			_ = err
			generatedUsersID[i] = user.ID
		}

		groupRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 4, true)

		members, err := r.GroupMember.Insert(t.Context(), group.ID, groupRole.ID, generatedUsersID[:4]...)

		require.Nil(t, err)

		expectedIDs := generatedUsersID[:4]
		for _, member := range members {
			require.Contains(t, expectedIDs, member.UserID)
		}
	})
}

// TestRepository_GroupMemberInsert_JoinedAtFilled проверяет, что при добавлении участника
// поле joined_at заполняется значением по умолчанию (§7 дизайна эпика Э2, миграция
// alter_group_members_add_joined_at).
func TestRepository_GroupMemberInsert_JoinedAtFilled(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		before := time.Now().UTC().Add(-time.Second)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		groupRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 4, true)

		members, err := r.GroupMember.Insert(t.Context(), group.ID, groupRole.ID, user.ID)
		require.NoError(t, err)
		require.Len(t, members, 1)

		after := time.Now().UTC().Add(time.Second)

		require.False(t, members[0].JoinedAt.IsZero())
		require.True(t, members[0].JoinedAt.After(before))
		require.True(t, members[0].JoinedAt.Before(after))
	})
}

func TestRepository_GroupMemberSelectByGroupID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		accountRole, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
		groupRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 4, true)

		groupOne, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		groupTwo, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())

		userOne, _ := r.User.Insert(
			t.Context(), f.Person().FirstName(), f.Person().LastName(), f.Hash().MD5(),
			f.Person().Contact().Email, accountRole.ID,
		)
		userTwo, _ := r.User.Insert(
			t.Context(), f.Person().FirstName(), f.Person().LastName(), f.Hash().MD5(),
			f.Person().Contact().Email, accountRole.ID,
		)

		_, err := r.GroupMember.Insert(t.Context(), groupOne.ID, groupRole.ID, userOne.ID, userTwo.ID)
		require.NoError(t, err)
		_, err = r.GroupMember.Insert(t.Context(), groupTwo.ID, groupRole.ID, userOne.ID)
		require.NoError(t, err)

		members, err := r.GroupMember.SelectByGroupID(t.Context(), groupOne.ID)

		require.NoError(t, err)
		require.Len(t, members, 2)

		expectedUserIDs := []uuid.UUID{userOne.ID, userTwo.ID}
		for _, member := range members {
			require.Equal(t, groupOne.ID, member.GroupID)
			require.Contains(t, expectedUserIDs, member.UserID)
		}
	})
}

func TestRepository_GroupMemberSelectByGroupID_EmptyNotNil(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		members, err := r.GroupMember.SelectByGroupID(t.Context(), uuid.New())

		require.NoError(t, err)
		require.NotNil(t, members)
		require.Empty(t, members)
	})
}

func TestRepository_GroupMemberUpdateRole_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
		user, _ := r.User.Insert(
			t.Context(), f.Person().FirstName(), f.Person().LastName(), f.Hash().MD5(),
			f.Person().Contact().Email, accountRole.ID,
		)
		oldRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 4, true)
		newRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 6, false)

		_, err := r.GroupMember.Insert(t.Context(), group.ID, oldRole.ID, user.ID)
		require.NoError(t, err)

		member, err := r.GroupMember.UpdateRole(t.Context(), group.ID, user.ID, newRole.ID)

		require.NoError(t, err)
		require.Equal(t, group.ID, member.GroupID)
		require.Equal(t, user.ID, member.UserID)
		require.Equal(t, newRole.ID, member.RoleID)

		stored, err := r.GroupMember.SelectByUserIDAndGroupID(t.Context(), user.ID, group.ID)
		require.NoError(t, err)
		require.Equal(t, newRole.ID, stored.RoleID)
	})
}

func TestRepository_GroupMemberUpdateRole_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		member, err := r.GroupMember.UpdateRole(t.Context(), uuid.New(), uuid.New(), uuid.New())

		require.ErrorIs(t, err, repository.ErrNotFound)
		require.Equal(t, domain.GroupMember{}, member)
	})
}

func TestRepository_GroupMemberSelectByUserIDAndGroupID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		groupRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 4, true)

		_, err := r.GroupMember.Insert(t.Context(), group.ID, groupRole.ID, user.ID)
		require.Nil(t, err)

		member, err := r.GroupMember.SelectByUserIDAndGroupID(t.Context(), user.ID, group.ID)

		require.Nil(t, err)
		require.Equal(t, user.ID, member.UserID)
		require.Equal(t, group.ID, member.GroupID)
		require.Equal(t, groupRole.ID, member.RoleID)
	})
}

func TestRepository_GroupMemberSelectByUserIDAndGroupID_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		member, err := r.GroupMember.SelectByUserIDAndGroupID(t.Context(), uuid.New(), uuid.New())

		require.NotNil(t, err)
		require.Equal(t, domain.GroupMember{}, member)
	})
}

func TestRepository_GroupMemberSelectByUserID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		accountRole, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)

		groupOne, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		groupTwo, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		groupRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 4, true)

		_, err := r.GroupMember.Insert(t.Context(), groupOne.ID, groupRole.ID, user.ID)
		require.NoError(t, err)
		_, err = r.GroupMember.Insert(t.Context(), groupTwo.ID, groupRole.ID, user.ID)
		require.NoError(t, err)

		members, err := r.GroupMember.SelectByUserID(t.Context(), user.ID)

		require.NoError(t, err)
		require.Len(t, members, 2)

		expectedGroupIDs := []uuid.UUID{groupOne.ID, groupTwo.ID}
		for _, member := range members {
			require.Equal(t, user.ID, member.UserID)
			require.Contains(t, expectedGroupIDs, member.GroupID)
		}
	})
}

func TestRepository_GroupMemberSelectByUserID_EmptyNotNil(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		members, err := r.GroupMember.SelectByUserID(t.Context(), uuid.New())

		require.NoError(t, err)
		require.NotNil(t, members)
		require.Empty(t, members)
	})
}
