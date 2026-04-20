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

func TestRepository_VideoInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)

		video, err := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusUploading)

		require.Nil(t, err)
		require.NotEmpty(t, video.ID)
		require.Equal(t, domain.VideoStatusUploading, video.Status)
		require.Equal(t, group.ID, video.GroupID)
		require.Equal(t, user.ID, video.Author)
	})
}

func TestRepository_VideoSelect_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		createdVideo, _ := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady)

		video, err := r.Video.Select(t.Context(), createdVideo.ID)

		require.Nil(t, err)
		require.Equal(t, createdVideo.ID, video.ID)
		require.Equal(t, domain.VideoStatusReady, video.Status)
	})
}

func TestRepository_VideoSelect_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video, err := r.Video.Select(t.Context(), uuid.New())

		require.NotNil(t, err)
		require.Equal(t, domain.Video{}, video)
	})
}

func TestRepository_VideoUpdate_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		video, _ := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusUploading)

		newStatus := domain.VideoStatusReady
		updatedVideo, err := r.Video.Update(t.Context(), video.ID, &newStatus)

		require.Nil(t, err)
		require.Equal(t, domain.VideoStatusReady, updatedVideo.Status)
	})
}

func TestRepository_VideoAssetCreate_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		video, _ := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady)

		asset, err := r.VideoAsset.Create(t.Context(), video.ID, domain.VideoAssetTagOriginal, "bucket", "video/mp4", 1024)

		require.Nil(t, err)
		require.NotEmpty(t, asset.FileID)
		require.Equal(t, video.ID, asset.VideoID)
		require.Equal(t, domain.VideoAssetTagOriginal, asset.Tag)
	})
}

func TestRepository_VideoAssetSelect_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		accountRole, _ := r.AccountRole.Insert(
			t.Context(),
			account.ID,
			f.Beer().Name(),
			nil,
			4,
			true,
			false,
		)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		video, _ := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady)
		createdAsset, _ := r.VideoAsset.Create(t.Context(), video.ID, domain.VideoAssetTagOriginal, "bucket", "video/mp4", 1024)

		assets, err := r.VideoAsset.Select(t.Context(), video.ID)

		require.Nil(t, err)
		require.Len(t, assets, 1)
		require.Equal(t, createdAsset.FileID, assets[0].FileID)
	})
}

func TestRepository_VideoAssetSelect_Empty(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assets, err := r.VideoAsset.Select(t.Context(), uuid.New())

		require.Nil(t, err)
		require.Len(t, assets, 0)
	})
}
