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

func TestRepository_GroupRoleInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			permission domain.PermissionMask = 3
			isDefault                        = true
			name                             = f.Beer().Name()
		)
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)

		role, err := r.GroupRole.Insert(t.Context(), account.ID, name, permission, isDefault)

		require.Nil(t, err)
		require.NotEmpty(t, role.ID)
		require.Equal(t, isDefault, role.IsDefault)
		require.Equal(t, permission, role.PermissionMask)
		require.Equal(t, name, role.Name)
		require.Equal(t, account.ID, role.AccountID)
	})
}

func TestRepository_GroupRoleSelectByAccount_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			accountCount                       = 7
			roleCount                          = 5
			permission   domain.PermissionMask = 3
			isDefault                          = true
		)

		accounts := make([]domain.Account, accountCount)
		generatedRoles := make([][]domain.GroupRole, accountCount)
		for i := range accountCount {
			accounts[i], _ = r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
			generatedRoles[i] = make([]domain.GroupRole, roleCount)
			for j := range roleCount {
				generatedRoles[i][j], _ = r.GroupRole.Insert(
					t.Context(),
					accounts[i].ID,
					f.Beer().Name(),
					permission,
					isDefault,
				)
			}
		}

		roles, err := r.GroupRole.SelectByAccount(t.Context(), accounts[5].ID)

		require.Nil(t, err)
		require.Len(t, roles, roleCount)

		expectedIDs := make([]uuid.UUID, roleCount)
		for i, role := range generatedRoles[5] {
			expectedIDs[i] = role.ID
		}

		for _, role := range roles {
			require.Contains(t, expectedIDs, role.ID)
		}
	})
}

func TestRepository_GroupRoleSelectByAccount_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		roles, err := r.GroupRole.SelectByAccount(t.Context(), uuid.New())

		require.Nil(t, roles)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

func TestRepository_GroupRoleSelectByID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			permission domain.PermissionMask = 5
			isDefault                        = false
			name                             = f.Beer().Name()
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		createdRole, _ := r.GroupRole.Insert(t.Context(), account.ID, name, permission, isDefault)

		roles, err := r.GroupRole.SelectByID(t.Context(), createdRole.ID)

		require.Nil(t, err)
		require.Len(t, roles, 1)
		require.Equal(t, createdRole.ID, roles[0].ID)
		require.Equal(t, name, roles[0].Name)
		require.Equal(t, permission, roles[0].PermissionMask)
		require.Equal(t, isDefault, roles[0].IsDefault)
	})
}

func TestRepository_GroupRoleSelectByID_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		roles, err := r.GroupRole.SelectByID(t.Context(), uuid.New())

		require.Nil(t, roles)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

func TestRepository_GroupRoleGetDefault_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)

		defaultRole, _ := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 1, true)
		_, _ = r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 2, false)

		role, err := r.GroupRole.GetDefault(t.Context(), account.ID)

		require.Nil(t, err)
		require.Equal(t, defaultRole.ID, role.ID)
		require.True(t, role.IsDefault)
	})
}

func TestRepository_GroupRoleGetDefault_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		_, err := r.GroupRole.GetDefault(t.Context(), uuid.New())

		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}
