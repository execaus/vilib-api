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

func TestRepository_UserGroupInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			name = f.Beer().Name()
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, err := r.UserGroup.Insert(t.Context(), account.ID, name)

		require.Nil(t, err)

		require.NotEmpty(t, group.ID)
		require.Equal(t, name, group.Name)
		require.Equal(t, account.ID, group.AccountID)

	})
}

func TestRepository_UserGroupSelectByID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const (
			accountCount   = 3
			userGroupCount = 5
		)

		generatedAccount := make([]domain.Account, accountCount)
		generatedUserGroupsID := make([][]uuid.UUID, userGroupCount)

		for i := range accountCount {
			generatedAccount[i], _ = r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)

			generatedUserGroupsID[i] = make([]uuid.UUID, userGroupCount)
			for j := range userGroupCount {
				userGroup, _ := r.UserGroup.Insert(t.Context(), generatedAccount[i].ID, f.Company().Name())
				generatedUserGroupsID[i][j] = userGroup.ID
			}
		}

		userGroups, err := r.UserGroup.GetByID(t.Context(), generatedUserGroupsID[2]...)

		require.Nil(t, err)
		require.Len(t, userGroups, userGroupCount)

		expectedIDs := generatedUserGroupsID[2]

		for _, role := range userGroups {
			require.Contains(t, expectedIDs, role.ID)
		}
	})
}

func TestRepository_UserGroupSelectByID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		roles, err := r.UserGroup.GetByID(t.Context(), uuid.New())

		require.Nil(t, roles)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}
