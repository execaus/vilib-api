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
