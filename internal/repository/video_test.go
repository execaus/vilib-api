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

// newTestVideo создаёт аккаунт, группу, роль, пользователя и видео для теста репозитория видео.
func newTestVideo(
	t *testing.T,
	r *repository.Repository,
	f faker.Faker,
	status domain.VideoStatus,
) domain.Video {
	t.Helper()

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

	video, err := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, status)
	require.NoError(t, err)

	return video
}

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

		require.NoError(t, err)
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

		require.NoError(t, err)
		require.Equal(t, createdVideo.ID, video.ID)
		require.Equal(t, domain.VideoStatusReady, video.Status)
	})
}

func TestRepository_VideoSelect_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video, err := r.Video.Select(t.Context(), uuid.New())

		require.Nil(t, video)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

func TestRepository_VideoUpdateStatusIf_MatchingFrom_UpdatesRowAndReturnsTrue(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusUploading)

		attempt := 1
		updated, err := r.Video.UpdateStatusIf(
			t.Context(),
			video.ID,
			[]domain.VideoStatus{domain.VideoStatusUploading},
			domain.VideoStatusQueued,
			domain.VideoPatch{ProcessingAttempt: &attempt},
		)

		require.NoError(t, err)
		require.True(t, updated)

		got, err := r.Video.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusQueued, got.Status)
		require.Equal(t, 1, got.ProcessingAttempt)
		require.True(
			t,
			got.StatusChangedAt.After(video.StatusChangedAt) || got.StatusChangedAt.Equal(video.StatusChangedAt),
		)
	})
}

func TestRepository_VideoUpdateStatusIf_MismatchedFrom_ReturnsFalseAndKeepsRow(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)

		updated, err := r.Video.UpdateStatusIf(
			t.Context(),
			video.ID,
			[]domain.VideoStatus{domain.VideoStatusUploading},
			domain.VideoStatusQueued,
			domain.VideoPatch{},
		)

		require.NoError(t, err)
		require.False(t, updated)

		got, err := r.Video.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusReady, got.Status)
	})
}

func TestRepository_VideoUpdateStatusIf_MismatchedExpectedAttempt_ReturnsFalse(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusQueued)

		wrongAttempt := 42
		updated, err := r.Video.UpdateStatusIf(
			t.Context(),
			video.ID,
			[]domain.VideoStatus{domain.VideoStatusQueued},
			domain.VideoStatusCompressing,
			domain.VideoPatch{ExpectedAttempt: &wrongAttempt},
		)

		require.NoError(t, err)
		require.False(t, updated)

		got, err := r.Video.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusQueued, got.Status)
	})
}

func TestRepository_VideoAssetInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)

		objectKey := "videos/" + video.ID.String() + "/original"
		asset, err := r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			objectKey,
			"video/mp4",
			1024,
		)

		require.NoError(t, err)
		require.NotEmpty(t, asset.FileID)
		require.Equal(t, video.ID, asset.VideoID)
		require.Equal(t, domain.VideoAssetKindOriginal, asset.Kind)
		require.Equal(t, domain.VideoProfile(""), asset.Profile)
		require.Equal(t, "bucket", asset.Bucket)
		require.Equal(t, objectKey, asset.ObjectKey)
		require.Equal(t, "video/mp4", asset.ContentType)
		require.Equal(t, int64(1024), asset.SizeBytes)
	})
}

func TestRepository_VideoAssetInsert_DuplicateKindAndProfile_ReturnsError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		objectKey := "videos/" + video.ID.String() + "/original"

		_, err := r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			objectKey,
			"video/mp4",
			1024,
		)
		require.NoError(t, err)

		_, err = r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			"videos/"+video.ID.String()+"/other",
			"video/mp4",
			2048,
		)

		require.Error(t, err)
	})
}

func TestRepository_VideoAssetSelect_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusReady)
		objectKey := "videos/" + video.ID.String() + "/original"
		createdAsset, _ := r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			objectKey,
			"video/mp4",
			1024,
		)

		assets, err := r.VideoAsset.Select(t.Context(), video.ID)

		require.NoError(t, err)
		require.Len(t, assets, 1)
		require.Equal(t, createdAsset.FileID, assets[0].FileID)
		require.Equal(t, createdAsset.ObjectKey, assets[0].ObjectKey)
	})
}

func TestRepository_VideoAssetSelect_Empty(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		assets, err := r.VideoAsset.Select(t.Context(), uuid.New())

		require.NoError(t, err)
		require.Empty(t, assets)
	})
}

// TestRepository_VideoAssetDeleteByVideoAndKinds_DeletesOnlyRequestedKinds проверяет, что
// удаляются только ассеты указанных видов вместе со связанными файлами, а оригинал остаётся
// (Э1-Т14: идемпотентная перерегистрация результатов обработки не должна трогать оригинал).
func TestRepository_VideoAssetDeleteByVideoAndKinds_DeletesOnlyRequestedKinds(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusCompressing)

		original, err := r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			"videos/"+video.ID.String()+"/original",
			"video/mp4",
			1024,
		)
		require.NoError(t, err)

		master, err := r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindHLSMaster,
			domain.VideoProfile(""),
			"bucket",
			"videos/"+video.ID.String()+"/hls/master.m3u8",
			"application/vnd.apple.mpegurl",
			512,
		)
		require.NoError(t, err)

		variant, err := r.VideoAsset.Insert(
			t.Context(),
			video.ID,
			domain.VideoAssetKindHLSVariant,
			domain.VideoProfile("720p"),
			"bucket",
			"videos/"+video.ID.String()+"/hls/720p/playlist.m3u8",
			"application/vnd.apple.mpegurl",
			2048,
		)
		require.NoError(t, err)

		err = r.VideoAsset.DeleteByVideoAndKinds(
			t.Context(),
			video.ID,
			[]domain.VideoAssetKind{domain.VideoAssetKindHLSMaster, domain.VideoAssetKindHLSVariant},
		)
		require.NoError(t, err)

		remaining, err := r.VideoAsset.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.Len(t, remaining, 1)
		require.Equal(t, original.FileID, remaining[0].FileID)
		require.Equal(t, domain.VideoAssetKindOriginal, remaining[0].Kind)

		remainingIDs := []uuid.UUID{remaining[0].FileID}
		require.NotContains(t, remainingIDs, master.FileID)
		require.NotContains(t, remainingIDs, variant.FileID)
	})
}

// TestRepository_VideoAssetDeleteByVideoAndKinds_NoMatchingAssets_NoError проверяет, что
// удаление при отсутствии ассетов указанных видов у видео не возвращает ошибку.
func TestRepository_VideoAssetDeleteByVideoAndKinds_NoMatchingAssets_NoError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusQueued)

		err := r.VideoAsset.DeleteByVideoAndKinds(
			t.Context(),
			video.ID,
			[]domain.VideoAssetKind{domain.VideoAssetKindHLSMaster, domain.VideoAssetKindHLSVariant},
		)

		require.NoError(t, err)
	})
}
