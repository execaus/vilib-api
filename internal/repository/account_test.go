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

func TestRepository_AccountInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		name := f.Person().Name()
		email := f.Person().Contact().Email
		startTime := time.Now()

		account, err := r.Account.Insert(t.Context(), name, email)

		require.Nil(t, err)
		require.NotEmpty(t, account.ID)
		require.Equal(t, email, account.Email)
		require.Equal(t, name, account.Name)
		require.WithinDuration(t, startTime, account.CreatedAt, time.Second)
	})
}

func TestRepository_AccountSelectByID_Success(t *testing.T) {
	t.Parallel()

	const (
		accountCount = 10
		selectCount  = 3
	)

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		generationAccounts := make([]domain.Account, accountCount)
		accountsID := make([]uuid.UUID, accountCount)
		for i := range accountCount {
			account, _ := r.Account.Insert(t.Context(), f.Person().Name(), f.Person().Contact().Email)
			generationAccounts[i] = account
			accountsID[i] = account.ID
		}

		accounts, err := r.Account.SelectByID(t.Context(), accountsID[:selectCount]...)

		require.Nil(t, err)
		require.Len(t, accounts, selectCount)

		for i, account := range accounts {
			require.Equal(t, generationAccounts[i].ID, account.ID)
			require.Equal(t, generationAccounts[i].Name, account.Name)
		}
	})
}

func TestRepository_AccountSelectByID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		accounts, err := r.Account.SelectByID(t.Context(), uuid.New())
		require.Nil(t, accounts)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

func TestRepository_AccountSelectByUsersID_Success(t *testing.T) {
	t.Parallel()

	const (
		accountCount       = 10
		roleInAccountCount = 3
		userInAccountCount = 5
	)

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		generatedAccounts := make([]domain.Account, accountCount)

		// создать аккаунты
		for i := range accountCount {
			acc, _ := r.Account.Insert(t.Context(), f.Person().Name(), f.Person().Contact().Email)
			generatedAccounts[i] = acc
		}

		// создать роли
		roles := make([][]domain.AccountRole, accountCount)
		for i, account := range generatedAccounts {
			roles[i] = make([]domain.AccountRole, roleInAccountCount)
			for j := range roleInAccountCount {
				roles[i][j], _ = r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 0, false)
			}
		}

		// создать пользователей
		users := make([][]domain.User, accountCount)
		for i, _ := range generatedAccounts {
			users[i] = make([]domain.User, userInAccountCount)
			for j := range userInAccountCount {
				users[i][j], _ = r.User.Insert(
					t.Context(),
					f.Person().FirstName(),
					f.Person().LastName(),
					f.Hash().MD5(),
					f.Person().Contact().Email,
					roles[i][j%roleInAccountCount].ID,
				)
			}
		}

		// проверка метода
		accounts, _ := r.Account.SelectByUsersID(
			t.Context(),
			users[0][0].ID,
			users[0][3].ID,
			users[4][0].ID,
			users[4][4].ID,
			users[7][0].ID,
			users[9][0].ID,
			users[9][0].ID,
		)
		actualIDs := make([]uuid.UUID, len(accounts))
		for i, account := range accounts {
			actualIDs[i] = account.ID
		}

		require.Len(t, accounts, 4)
		expectedIDs := []uuid.UUID{
			generatedAccounts[0].ID,
			generatedAccounts[4].ID,
			generatedAccounts[7].ID,
			generatedAccounts[9].ID,
		}
		for _, id := range expectedIDs {
			require.Contains(t, actualIDs, id)
		}
	})
}

func TestRepository_AccountSelectByUsersID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		accounts, err := r.Account.SelectByUsersID(t.Context(), uuid.New())
		require.Nil(t, accounts)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}
