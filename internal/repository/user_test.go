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

func TestRepository_UserSelectByIDs_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const userCount = 7

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		role, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)

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

		requestedIDs := generatedUsersID[:5]
		users, err := r.User.SelectByIDs(t.Context(), requestedIDs)

		require.NoError(t, err)
		require.Len(t, users, 5)

		for _, user := range users {
			require.Contains(t, requestedIDs, user.ID)
		}
	})
}

// TestRepository_UserSelectByIDs_MissingIDsSkipped проверяет, что несуществующие
// идентификаторы просто отсутствуют в результате — это не ошибка (батч-выборка авторов
// видео, П-6 контракта Э2), в отличие от SelectByID.
func TestRepository_UserSelectByIDs_MissingIDsSkipped(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		role, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)

		user, _ := r.User.Insert(
			t.Context(), f.Person().FirstName(), f.Person().LastName(), f.Hash().MD5(),
			f.Person().Contact().Email, role.ID,
		)

		users, err := r.User.SelectByIDs(t.Context(), []uuid.UUID{user.ID, uuid.New()})

		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, user.ID, users[0].ID)
	})
}

func TestRepository_UserSelectByIDs_EmptyInput(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		users, err := r.User.SelectByIDs(t.Context(), nil)

		require.NoError(t, err)
		require.Empty(t, users)
	})
}

func TestRepository_UserSelectByEmailAndAccountID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		email := f.Person().Contact().Email

		accountOne, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		roleOne, _ := r.AccountRole.Insert(t.Context(), accountOne.ID, f.Beer().Name(), nil, 4, true, false)
		userOne, err := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			email,
			roleOne.ID,
		)
		require.NoError(t, err)

		accountTwo, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		roleTwo, _ := r.AccountRole.Insert(t.Context(), accountTwo.ID, f.Beer().Name(), nil, 4, true, false)
		_, err = r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			email,
			roleTwo.ID,
		)
		require.NoError(t, err)

		user, err := r.User.SelectByEmailAndAccountID(t.Context(), email, accountOne.ID)

		require.NoError(t, err)
		require.Equal(t, userOne.ID, user.ID)
		require.Equal(t, email, user.Email)
	})
}

func TestRepository_UserSelectByEmailAndAccountID_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user, err := r.User.SelectByEmailAndAccountID(t.Context(), f.Person().Contact().Email, uuid.New())

		require.ErrorIs(t, err, repository.ErrNotFound)
		require.Equal(t, domain.User{}, user)
	})
}

// insertTestUser — вспомогательная вставка пользователя с ролью для тестов UpdateProfile.
func insertTestUser(t *testing.T, r *repository.Repository, f faker.Faker) domain.User {
	t.Helper()

	account, err := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
	require.NoError(t, err)

	role, err := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 0, true, false)
	require.NoError(t, err)

	user, err := r.User.Insert(
		t.Context(),
		f.Person().FirstName(),
		f.Person().LastName(),
		f.Hash().MD5(),
		f.Person().Contact().Email,
		role.ID,
	)
	require.NoError(t, err)

	return user
}

func TestRepository_UserUpdateProfile_UpdatesBothFields(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)

		newName := f.Person().FirstName()
		newSurname := f.Person().LastName()

		updated, err := r.User.UpdateProfile(t.Context(), user.ID, &newName, &newSurname)

		require.NoError(t, err)
		require.Equal(t, user.ID, updated.ID)
		require.Equal(t, newName, updated.Name)
		require.Equal(t, newSurname, updated.Surname)
	})
}

func TestRepository_UserUpdateProfile_UpdatesOnlyName(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)

		newName := f.Person().FirstName()

		updated, err := r.User.UpdateProfile(t.Context(), user.ID, &newName, nil)

		require.NoError(t, err)
		require.Equal(t, newName, updated.Name)
		require.Equal(t, user.Surname, updated.Surname)
	})
}

func TestRepository_UserUpdateProfile_UpdatesOnlySurname(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)

		newSurname := f.Person().LastName()

		updated, err := r.User.UpdateProfile(t.Context(), user.ID, nil, &newSurname)

		require.NoError(t, err)
		require.Equal(t, user.Name, updated.Name)
		require.Equal(t, newSurname, updated.Surname)
	})
}

func TestRepository_UserUpdateProfile_BothNilReturnsCurrent(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)

		updated, err := r.User.UpdateProfile(t.Context(), user.ID, nil, nil)

		require.NoError(t, err)
		require.Equal(t, user, updated)
	})
}

func TestRepository_UserUpdateProfile_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		newName := f.Person().FirstName()

		_, err := r.User.UpdateProfile(t.Context(), uuid.New(), &newName, nil)

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}
