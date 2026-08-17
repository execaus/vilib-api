package repository_test

import (
	"testing"
	"time"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestRepository_PasswordResetTokenInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)
		email := user.Email
		hash := f.Hash().SHA256()
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)

		token, err := r.PasswordResetToken.Insert(t.Context(), user.ID, email, hash, expiresAt)

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, token.ID)
		require.Equal(t, user.ID, token.UserID)
		require.Equal(t, email, token.Email)
		require.Equal(t, hash, token.TokenHash)
		require.WithinDuration(t, expiresAt, token.ExpiresAt, time.Second)
		require.Nil(t, token.UsedAt)
		require.NotZero(t, token.CreatedAt)
	})
}

func TestRepository_PasswordResetTokenSelectByHash_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)
		hash := f.Hash().SHA256()
		expiresAt := time.Now().Add(time.Hour)

		inserted, err := r.PasswordResetToken.Insert(t.Context(), user.ID, user.Email, hash, expiresAt)
		require.NoError(t, err)

		got, err := r.PasswordResetToken.SelectByHash(t.Context(), hash)

		require.NoError(t, err)
		require.Equal(t, inserted.ID, got.ID)
		require.Equal(t, user.ID, got.UserID)
	})
}

func TestRepository_PasswordResetTokenSelectByHash_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		_, err := r.PasswordResetToken.SelectByHash(t.Context(), f.Hash().SHA256())

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_PasswordResetTokenMarkUsed_SetsUsedAt(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user := insertTestUser(t, r, f)
		hash := f.Hash().SHA256()

		inserted, err := r.PasswordResetToken.Insert(t.Context(), user.ID, user.Email, hash, time.Now().Add(time.Hour))
		require.NoError(t, err)

		err = r.PasswordResetToken.MarkUsed(t.Context(), inserted.ID)
		require.NoError(t, err)

		got, err := r.PasswordResetToken.SelectByHash(t.Context(), hash)
		require.NoError(t, err)
		require.NotNil(t, got.UsedAt)
	})
}

func TestRepository_PasswordResetTokenDeleteByEmail_RemovesOnlyMatchingEmail(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		user1 := insertTestUser(t, r, f)
		user2 := insertTestUser(t, r, f)

		hash1 := f.Hash().SHA256()
		hash2 := f.Hash().SHA256()

		_, err := r.PasswordResetToken.Insert(t.Context(), user1.ID, user1.Email, hash1, time.Now().Add(time.Hour))
		require.NoError(t, err)
		_, err = r.PasswordResetToken.Insert(t.Context(), user2.ID, user2.Email, hash2, time.Now().Add(time.Hour))
		require.NoError(t, err)

		err = r.PasswordResetToken.DeleteByEmail(t.Context(), user1.Email)
		require.NoError(t, err)

		_, err = r.PasswordResetToken.SelectByHash(t.Context(), hash1)
		require.ErrorIs(t, err, repository.ErrNotFound)

		got, err := r.PasswordResetToken.SelectByHash(t.Context(), hash2)
		require.NoError(t, err)
		require.Equal(t, user2.ID, got.UserID)
	})
}
