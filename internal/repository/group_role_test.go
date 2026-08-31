package repository_test

import (
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
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
			name                             = testutil.UniqueName(f)
		)
		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)

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
			account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
			require.NoError(t, err)
			accounts[i] = account

			generatedRoles[i] = make([]domain.GroupRole, roleCount)
			for j := range roleCount {
				role, insertErr := r.GroupRole.Insert(
					t.Context(),
					accounts[i].ID,
					testutil.UniqueName(f),
					permission,
					isDefault,
				)
				require.NoError(t, insertErr)
				generatedRoles[i][j] = role
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
			name                             = testutil.UniqueName(f)
		)

		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		createdRole, err := r.GroupRole.Insert(t.Context(), account.ID, name, permission, isDefault)
		require.NoError(t, err)

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
		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)

		defaultRole, err := r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 1, true)
		require.NoError(t, err)
		_, err = r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 2, false)
		require.NoError(t, err)

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

func TestRepository_GroupRoleUpdate_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			newName                             = testutil.UniqueName(f)
			newPermission domain.PermissionMask = 6
		)

		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		role, err := r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 1, false)
		require.NoError(t, err)

		updated, err := r.GroupRole.Update(t.Context(), role.ID, newName, newPermission, true)

		require.NoError(t, err)
		require.Equal(t, role.ID, updated.ID)
		require.Equal(t, newName, updated.Name)
		require.Equal(t, newPermission, updated.PermissionMask)
		require.True(t, updated.IsDefault)

		roles, err := r.GroupRole.SelectByID(t.Context(), role.ID)
		require.NoError(t, err)
		require.Equal(t, updated, roles[0])
	})
}

func TestRepository_GroupRoleUpdate_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		_, err := r.GroupRole.Update(t.Context(), uuid.New(), testutil.UniqueName(f), 0, false)

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_GroupRoleUpdate_DuplicateNameReturnsError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		existing, err := r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 0, false)
		require.NoError(t, err)
		role, err := r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 0, false)
		require.NoError(t, err)

		_, err = r.GroupRole.Update(t.Context(), role.ID, existing.Name, 0, false)

		require.ErrorIs(t, dberrors.GroupRoleErrors.ErrUniqueGroupRolesAccountIdNameKey, err)
	})
}

func TestRepository_GroupRoleClearDefault_UnsetsFlagForAllAccountRoles(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		otherAccount, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)

		roleA, err := r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 0, true)
		require.NoError(t, err)
		roleB, err := r.GroupRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), 0, false)
		require.NoError(t, err)
		otherRole, err := r.GroupRole.Insert(t.Context(), otherAccount.ID, testutil.UniqueName(f), 0, true)
		require.NoError(t, err)

		err = r.GroupRole.ClearDefault(t.Context(), account.ID)
		require.NoError(t, err)

		roles, err := r.GroupRole.SelectByID(t.Context(), roleA.ID)
		require.NoError(t, err)
		require.False(t, roles[0].IsDefault)

		roles, err = r.GroupRole.SelectByID(t.Context(), roleB.ID)
		require.NoError(t, err)
		require.False(t, roles[0].IsDefault)

		// Роль по умолчанию другого аккаунта не затрагивается.
		otherRoles, err := r.GroupRole.SelectByID(t.Context(), otherRole.ID)
		require.NoError(t, err)
		require.True(t, otherRoles[0].IsDefault)
	})
}
