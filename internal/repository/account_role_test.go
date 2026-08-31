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

func TestRepository_AccountRoleInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			permission domain.PermissionMask = 5
			name                             = testutil.UniqueName(f)
			isDefault                        = true
			isSystem                         = true
		)

		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)

		parent, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 0, false, false)
		require.NoError(t, err)
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
			account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
			require.NoError(t, err)
			accounts[i] = account

			generatedRoles[i] = make([]domain.AccountRole, roleInAccountCount)
			for j := range roleInAccountCount {
				role, insertErr := r.AccountRole.Insert(
					t.Context(),
					accounts[i].ID,
					testutil.UniqueName(f),
					nil,
					0,
					false,
					false,
				)
				require.NoError(t, insertErr)
				generatedRoles[i][j] = role
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
			account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
			require.NoError(t, err)
			accounts[i] = account

			generatedRoles[i] = make([]domain.AccountRole, roleInAccountCount)
			for j := range roleInAccountCount {
				role, insertErr := r.AccountRole.Insert(
					t.Context(),
					accounts[i].ID,
					testutil.UniqueName(f),
					nil,
					0,
					false,
					false,
				)
				require.NoError(t, insertErr)
				generatedRoles[i][j] = role
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

func TestRepository_AccountRoleUpdate_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			newName                             = testutil.UniqueName(f)
			newPermission domain.PermissionMask = 7
		)

		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		parent, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 0, false, false)
		require.NoError(t, err)
		role, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 1, false, false)
		require.NoError(t, err)

		updated, err := r.AccountRole.Update(t.Context(), role.ID, newName, &parent.ID, newPermission, true)

		require.NoError(t, err)
		require.Equal(t, role.ID, updated.ID)
		require.Equal(t, newName, updated.Name)
		require.Equal(t, newPermission, updated.PermissionMask)
		require.Equal(t, parent.ID, *updated.ParentID)
		require.True(t, updated.IsDefault)

		roles, err := r.AccountRole.SelectByID(t.Context(), role.ID)
		require.NoError(t, err)
		require.Equal(t, updated, roles[0])
	})
}

func TestRepository_AccountRoleUpdate_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		_, err := r.AccountRole.Update(t.Context(), uuid.New(), testutil.UniqueName(f), nil, 0, false)

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_AccountRoleUpdate_DuplicateNameReturnsError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		existing, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 0, false, false)
		require.NoError(t, err)
		role, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 0, false, false)
		require.NoError(t, err)

		_, err = r.AccountRole.Update(t.Context(), role.ID, existing.Name, nil, 0, false)

		require.ErrorIs(t, dberrors.AccountRoleErrors.ErrUniqueUniqueAccountRole, err)
	})
}

func TestRepository_AccountRoleClearDefault_UnsetsFlagForAllAccountRoles(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)
		otherAccount, err := r.Account.Insert(t.Context(), testutil.UniqueName(f), f.Person().Contact().Email)
		require.NoError(t, err)

		roleA, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 0, true, false)
		require.NoError(t, err)
		roleB, err := r.AccountRole.Insert(t.Context(), account.ID, testutil.UniqueName(f), nil, 0, false, false)
		require.NoError(t, err)
		otherRole, err := r.AccountRole.Insert(
			t.Context(), otherAccount.ID, testutil.UniqueName(f), nil, 0, true, false,
		)
		require.NoError(t, err)

		err = r.AccountRole.ClearDefault(t.Context(), account.ID)
		require.NoError(t, err)

		roles, err := r.AccountRole.SelectByID(t.Context(), roleA.ID, roleB.ID)
		require.NoError(t, err)
		for _, role := range roles {
			require.False(t, role.IsDefault)
		}

		// Роль по умолчанию другого аккаунта не затрагивается.
		otherRoles, err := r.AccountRole.SelectByID(t.Context(), otherRole.ID)
		require.NoError(t, err)
		require.True(t, otherRoles[0].IsDefault)
	})
}
