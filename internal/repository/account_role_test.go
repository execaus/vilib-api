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

func TestRepository_AccountRoleInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			permission domain.PermissionMask = 5
			name                             = f.Beer().Name()
			isDefault                        = true
			isSystem                         = true
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)

		parent, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 0, false, false)
		role, err := r.AccountRole.Insert(t.Context(), account.ID, name, &parent.ID, permission, isDefault, isSystem)

		require.Nil(t, err)
		require.NotEmpty(t, role.ID)
		require.Equal(t, permission, role.PermissionMask)
		require.Equal(t, name, role.Name)
		require.Equal(t, isDefault, role.IsDefault)
		require.Equal(t, parent.ID, *role.ParentID)
		require.Equal(t, isSystem, role.IsSystem)
	})
}

func TestRepository_AccountRoleSelectByAccountID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const (
			accountCount       = 5
			roleInAccountCount = 3
		)

		accounts := make([]domain.Account, accountCount)
		generatedRoles := make([][]domain.AccountRole, accountCount)
		for i := range accountCount {
			accounts[i], _ = r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
			generatedRoles[i] = make([]domain.AccountRole, roleInAccountCount)
			for j := range roleInAccountCount {
				generatedRoles[i][j], _ = r.AccountRole.Insert(
					t.Context(),
					accounts[i].ID,
					f.Beer().Name(),
					nil,
					0,
					false,
					false,
				)
			}
		}

		roles, err := r.AccountRole.SelectByAccountID(t.Context(), accounts[3].ID)

		require.Nil(t, err)
		require.Len(t, roles, roleInAccountCount)
		for _, role := range roles {
			require.Contains(t, generatedRoles[3], role)
		}
	})
}

func TestRepository_AccountRoleSelectByAccountID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		roles, err := r.AccountRole.SelectByAccountID(t.Context(), uuid.New())

		require.Nil(t, roles)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

func TestRepository_AccountRoleSelectByID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const (
			accountCount       = 5
			roleInAccountCount = 3
		)

		accounts := make([]domain.Account, accountCount)
		generatedRoles := make([][]domain.AccountRole, accountCount)
		for i := range accountCount {
			accounts[i], _ = r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
			generatedRoles[i] = make([]domain.AccountRole, roleInAccountCount)
			for j := range roleInAccountCount {
				generatedRoles[i][j], _ = r.AccountRole.Insert(
					t.Context(),
					accounts[i].ID,
					f.Beer().Name(),
					nil,
					0,
					false,
					false,
				)
			}
		}

		roles, err := r.AccountRole.SelectByID(t.Context(), generatedRoles[3][0].ID)

		require.Nil(t, err)
		require.Contains(t, generatedRoles[3], roles[0])
	})
}

func TestRepository_AccountRoleSelectByID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		roles, err := r.AccountRole.SelectByID(t.Context(), uuid.New())

		require.Nil(t, roles)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}
