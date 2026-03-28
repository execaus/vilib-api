package repository_test

import (
	"testing"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

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
