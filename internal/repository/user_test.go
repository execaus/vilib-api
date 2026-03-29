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

func TestRepository_UserSelectByEmail_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const (
			emailCount         = 5
			userWithEmailCount = 7
			permission         = 3
		)

		emails := make([]string, emailCount)
		generatedUsers := make([][]domain.User, emailCount)
		for i := range emailCount {
			emails[i] = f.Person().Contact().Email
			account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
			role, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, permission, true, false)
			generatedUsers[i] = make([]domain.User, userWithEmailCount)
			for j := range userWithEmailCount {
				generatedUsers[i][j], _ = r.User.Insert(
					t.Context(),
					f.Person().FirstName(),
					f.Person().LastName(),
					f.Hash().MD5(),
					emails[i],
					role.ID,
				)
			}
		}

		users, err := r.User.SelectByEmail(t.Context(), emails[4])

		require.Nil(t, err)
		require.Len(t, users, userWithEmailCount)

		expectedIDs := make([]uuid.UUID, userWithEmailCount)
		for i, user := range generatedUsers[4] {
			expectedIDs[i] = user.ID
		}

		for _, user := range users {
			require.Contains(t, expectedIDs, user.ID)
		}
	})
}

func TestRepository_UserSelectByEmail_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		users, err := r.User.SelectByEmail(t.Context(), f.Person().Contact().Email)

		require.Nil(t, users)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

func TestRepository_UserInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			permission domain.PermissionMask = 3
			hash                             = f.Hash().MD5()
			email                            = f.Person().Contact().Email
			name                             = f.Person().FirstName()
			surname                          = f.Person().LastName()
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		role, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			permission,
			true,
			false,
		)

		timeStart := time.Now()
		user, err := r.User.Insert(t.Context(), name, surname, hash, email, role.ID)

		require.Nil(t, err)

		require.NotEmpty(t, user.ID)
		require.Equal(t, email, user.Email)
		require.Equal(t, name, user.Name)
		require.Equal(t, surname, user.Surname)
		require.Equal(t, role.ID, user.RoleID)
		require.Equal(t, hash, user.PasswordHash)
		require.WithinDuration(t, timeStart, user.CreatedAt, time.Second)
	})
}

func TestRepository_UserSelectByID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const (
			userCount = 7
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		role, _ := r.AccountRole.Insert(
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
			user, _ := r.User.Insert(
				t.Context(),
				f.Person().FirstName(),
				f.Person().LastName(),
				f.Hash().MD5(),
				f.Person().Contact().Email,
				role.ID,
			)
			generatedUsersID[i] = user.ID
		}

		users, err := r.User.SelectByID(t.Context(), generatedUsersID[:5]...)

		require.Nil(t, err)

		expectedIDs := generatedUsersID[:5]
		for _, user := range users {
			require.Contains(t, expectedIDs, user.ID)
		}
	})
}

func TestRepository_UserSelectByID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		users, err := r.User.SelectByID(t.Context(), uuid.New())

		require.Nil(t, users)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}
