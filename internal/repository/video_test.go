package repository_test

import (
	"testing"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

// newTestVideo создаёт аккаунт, группу, роль, пользователя и обычное (не срочное) видео для
// теста репозитория видео.
func newTestVideo(
	t *testing.T,
	r *repository.Repository,
	f faker.Faker,
	status domain.VideoStatus,
) domain.Video {
	t.Helper()

	return newTestVideoWithUrgency(t, r, f, status, false)
}

// newTestVideoWithUrgency — то же, что newTestVideo, но с явным указанием полосы обработки
// (эпик Э5, исправление Д-1: тесты watchdog'а по полосам).
func newTestVideoWithUrgency(
	t *testing.T,
	r *repository.Repository,
	f faker.Faker,
	status domain.VideoStatus,
	isUrgent bool,
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

	video, err := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, status, isUrgent)
	require.NoError(t, err)

	return video
}

// TestRepository_VideoInsert_PersistsIsUrgentFlag проверяет, что признак срочности (эпик Э5,
// В-2) сохраняется в БД и считывается обратно ровно тем значением, что было передано.
func TestRepository_VideoInsert_PersistsIsUrgentFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		isUrgent bool
	}{
		{name: "archive video is not urgent", isUrgent: false},
		{name: "urgent video keeps flag", isUrgent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

				video, err := r.Video.Insert(
					t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusUploading, tt.isUrgent,
				)

				require.NoError(t, err)
				require.NotEmpty(t, video.ID)
				require.Equal(t, domain.VideoStatusUploading, video.Status)
				require.Equal(t, group.ID, video.GroupID)
				require.Equal(t, user.ID, video.Author)
				require.Equal(t, tt.isUrgent, video.IsUrgent)

				got, err := r.Video.Select(t.Context(), video.ID)
				require.NoError(t, err)
				require.Equal(t, tt.isUrgent, got.IsUrgent)
			})
		})
	}
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
		createdVideo, _ := r.Video.Insert(
			t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady, false,
		)

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

// TestRepository_VideoUpdateStatusIf_QueuedAtPatch_SetsQueuedAt проверяет, что тот же условный
// UPDATE, что переводит uploading → queued, проставляет queued_at — момент complete из
// метрики времени публикации (эпик Э5, Э5-Т5).
func TestRepository_VideoUpdateStatusIf_QueuedAtPatch_SetsQueuedAt(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusUploading)
		require.Nil(t, video.QueuedAt)

		attempt := 1
		// Колонка queued_at — timestamp без часового пояса: как и в остальных тестах репозитория
		// (например, PasswordResetTokenInsert), сравнение ведётся в UTC, чтобы не зависеть от
		// локального часового пояса машины, на которой выполняется тест.
		queuedAt := time.Now().UTC()
		updated, err := r.Video.UpdateStatusIf(
			t.Context(),
			video.ID,
			[]domain.VideoStatus{domain.VideoStatusUploading},
			domain.VideoStatusQueued,
			domain.VideoPatch{ProcessingAttempt: &attempt, QueuedAt: &queuedAt},
		)

		require.NoError(t, err)
		require.True(t, updated)

		got, err := r.Video.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.NotNil(t, got.QueuedAt)
		require.WithinDuration(t, queuedAt, *got.QueuedAt, time.Second)
	})
}

// TestRepository_VideoUpdateStatusIf_CompressingStartedAtPatch_SetsCompressingStartedAt
// проверяет, что тот же условный UPDATE, что переводит queued → compressing по событию
// ProcessingStarted, проставляет compressing_started_at (эпик Э5, исправление Д-1).
func TestRepository_VideoUpdateStatusIf_CompressingStartedAtPatch_SetsCompressingStartedAt(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		video := newTestVideo(t, r, f, domain.VideoStatusQueued)
		require.Nil(t, video.CompressingStartedAt)

		attempt := video.ProcessingAttempt
		startedAt := time.Now().UTC()
		updated, err := r.Video.UpdateStatusIf(
			t.Context(),
			video.ID,
			[]domain.VideoStatus{domain.VideoStatusQueued},
			domain.VideoStatusCompressing,
			domain.VideoPatch{ExpectedAttempt: &attempt, CompressingStartedAt: &startedAt},
		)

		require.NoError(t, err)
		require.True(t, updated)

		got, err := r.Video.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.NotNil(t, got.CompressingStartedAt)
		require.WithinDuration(t, startedAt, *got.CompressingStartedAt, time.Second)
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

// TestRepository_VideoAssetSelectByVideoIDs_GroupsAssetsByVideoWithFileData проверяет, что
// SelectByVideoIDs одним батч-запросом возвращает ассеты нескольких видео вместе с данными
// связанных файлов (Bucket, ObjectKey, ContentType, SizeBytes), не затрагивая ассеты видео вне
// запрошенного списка (Э1-Т20).
func TestRepository_VideoAssetSelectByVideoIDs_GroupsAssetsByVideoWithFileData(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		videoA := newTestVideo(t, r, f, domain.VideoStatusReady)
		videoB := newTestVideo(t, r, f, domain.VideoStatusReady)
		videoC := newTestVideo(t, r, f, domain.VideoStatusReady)

		originalA, err := r.VideoAsset.Insert(
			t.Context(),
			videoA.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			"videos/"+videoA.ID.String()+"/original",
			"video/mp4",
			1024,
		)
		require.NoError(t, err)

		variantB, err := r.VideoAsset.Insert(
			t.Context(),
			videoB.ID,
			domain.VideoAssetKindHLSVariant,
			domain.VideoProfile("720p"),
			"bucket",
			"videos/"+videoB.ID.String()+"/hls/720p/playlist.m3u8",
			"application/vnd.apple.mpegurl",
			2048,
		)
		require.NoError(t, err)

		// Ассет видео C не входит в запрошенный список — не должен попасть в результат.
		_, err = r.VideoAsset.Insert(
			t.Context(),
			videoC.ID,
			domain.VideoAssetKindOriginal,
			domain.VideoProfile(""),
			"bucket",
			"videos/"+videoC.ID.String()+"/original",
			"video/mp4",
			4096,
		)
		require.NoError(t, err)

		assets, err := r.VideoAsset.SelectByVideoIDs(t.Context(), []uuid.UUID{videoA.ID, videoB.ID})
		require.NoError(t, err)
		require.Len(t, assets, 2)

		byVideo := make(map[uuid.UUID]domain.VideoAsset, len(assets))
		for _, asset := range assets {
			byVideo[asset.VideoID] = asset
		}

		gotA, ok := byVideo[videoA.ID]
		require.True(t, ok)
		require.Equal(t, originalA.FileID, gotA.FileID)
		require.Equal(t, domain.VideoAssetKindOriginal, gotA.Kind)
		require.Equal(t, "bucket", gotA.Bucket)
		require.Equal(t, "videos/"+videoA.ID.String()+"/original", gotA.ObjectKey)
		require.Equal(t, "video/mp4", gotA.ContentType)
		require.Equal(t, int64(1024), gotA.SizeBytes)

		gotB, ok := byVideo[videoB.ID]
		require.True(t, ok)
		require.Equal(t, variantB.FileID, gotB.FileID)
		require.Equal(t, domain.VideoAssetKindHLSVariant, gotB.Kind)
		require.Equal(t, domain.VideoProfile("720p"), gotB.Profile)
		require.Equal(t, int64(2048), gotB.SizeBytes)
	})
}

// TestRepository_VideoAssetSelectByVideoIDs_EmptyList_ReturnsEmptyWithoutQuery проверяет, что
// пустой список идентификаторов возвращает пустой результат без обращения к БД.
func TestRepository_VideoAssetSelectByVideoIDs_EmptyList_ReturnsEmptyWithoutQuery(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, _ faker.Faker) {
		assets, err := r.VideoAsset.SelectByVideoIDs(t.Context(), nil)

		require.NoError(t, err)
		require.Empty(t, assets)
	})
}

// backdateVideoTimestamp напрямую переписывает created_at/status_changed_at видео в обход
// репозитория — нужно для тестов watchdog'а, где строка должна выглядеть "зависшей" дольше
// таймаута. Прямой доступ к *bob.DB получают только тесты, использующие testutil.WithDB
// (не TestRepositoryWithDB) — это осознанное исключение для симуляции течения времени.
func backdateVideoTimestamp(t *testing.T, bobDB *bob.DB, videoID uuid.UUID, at time.Time) {
	t.Helper()

	_, err := bobDB.ExecContext(
		t.Context(),
		"UPDATE app.user_group_videos SET created_at = $1, status_changed_at = $1 WHERE id = $2",
		at, videoID,
	)
	require.NoError(t, err)
}

// TestRepository_VideoUpdateTimedOut_TranslatesOnlyOverdueRowsOfRequestedStatus проверяет,
// что UpdateTimedOut переводит в failed только строку заданного статуса, чья контрольная
// метка старше before — непросроченная строка того же статуса и просроченная строка другого
// статуса не затрагиваются (§8 дизайна эпика).
func TestRepository_VideoUpdateTimedOut_TranslatesOnlyOverdueRowsOfRequestedStatus(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		overdueUploading := newTestVideo(t, r, f, domain.VideoStatusUploading)
		freshUploading := newTestVideo(t, r, f, domain.VideoStatusUploading)
		overdueQueued := newTestVideo(t, r, f, domain.VideoStatusQueued)

		threshold := time.Now().Add(-2 * time.Hour)
		backdateVideoTimestamp(t, bobDB, overdueUploading.ID, threshold.Add(-time.Minute))
		backdateVideoTimestamp(t, bobDB, freshUploading.ID, threshold.Add(time.Minute))
		backdateVideoTimestamp(t, bobDB, overdueQueued.ID, threshold.Add(-time.Minute))

		failure := domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: "загрузка не завершена за 2h0m0s",
		}
		ids, err := r.Video.UpdateTimedOut(t.Context(), domain.VideoStatusUploading, threshold, failure, nil)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{overdueUploading.ID}, ids)

		got, err := r.Video.Select(t.Context(), overdueUploading.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusFailed, got.Status)
		require.NotNil(t, got.FailureClass)
		require.Equal(t, domain.VideoFailureClassTimeout, *got.FailureClass)
		require.NotNil(t, got.FailureReason)
		require.Equal(t, "загрузка не завершена за 2h0m0s", *got.FailureReason)
		require.True(t, got.StatusChangedAt.After(threshold))

		stillUploading, err := r.Video.Select(t.Context(), freshUploading.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusUploading, stillUploading.Status)

		stillQueued, err := r.Video.Select(t.Context(), overdueQueued.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusQueued, stillQueued.Status)
	})
}

// TestRepository_VideoUpdateTimedOut_RepeatedCall_Idempotent проверяет, что повторный вызов
// после первого перевода строки в failed больше не находит просроченных строк — watchdog
// безопасен при повторных тиках и при нескольких одновременно работающих инстансах API.
func TestRepository_VideoUpdateTimedOut_RepeatedCall_Idempotent(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		video := newTestVideo(t, r, f, domain.VideoStatusQueued)

		threshold := time.Now().Add(-time.Hour)
		backdateVideoTimestamp(t, bobDB, video.ID, threshold.Add(-time.Minute))

		failure := domain.VideoFailure{Class: domain.VideoFailureClassTimeout, Reason: "не взято в обработку"}

		first, err := r.Video.UpdateTimedOut(t.Context(), domain.VideoStatusQueued, threshold, failure, nil)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{video.ID}, first)

		second, err := r.Video.UpdateTimedOut(t.Context(), domain.VideoStatusQueued, threshold, failure, nil)
		require.NoError(t, err)
		require.Empty(t, second)
	})
}

// TestRepository_VideoUpdateTimedOut_FiltersByIsUrgent проверяет фильтр по полосе обработки
// (эпик Э5, исправление Д-1): при непустом isUrgent затрагиваются только просроченные видео
// указанной полосы, видео другой полосы с той же просроченной меткой не трогаются.
func TestRepository_VideoUpdateTimedOut_FiltersByIsUrgent(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		archival := newTestVideo(t, r, f, domain.VideoStatusQueued)
		urgent := newTestVideoWithUrgency(t, r, f, domain.VideoStatusQueued, true)

		threshold := time.Now().Add(-time.Hour)
		backdateVideoTimestamp(t, bobDB, archival.ID, threshold.Add(-time.Minute))
		backdateVideoTimestamp(t, bobDB, urgent.ID, threshold.Add(-time.Minute))

		failure := domain.VideoFailure{
			Class:  domain.VideoFailureClassTimeout,
			Reason: "конвейер срочной полосы не продвигается уже 15m0s",
		}
		urgentFilter := true
		ids, err := r.Video.UpdateTimedOut(t.Context(), domain.VideoStatusQueued, threshold, failure, &urgentFilter)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{urgent.ID}, ids)

		stillQueued, err := r.Video.Select(t.Context(), archival.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusQueued, stillQueued.Status)

		failed, err := r.Video.Select(t.Context(), urgent.ID)
		require.NoError(t, err)
		require.Equal(t, domain.VideoStatusFailed, failed.Status)
	})
}

// TestRepository_VideoDelete_RemovesAssetsAndFiles проверяет, что удаление видео убирает и
// связанные ассеты, и файлы — без сирот в БД (Э1-Т21).
func TestRepository_VideoDelete_RemovesAssetsAndFiles(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		video := newTestVideo(t, r, f, domain.VideoStatusReady)

		asset, err := r.VideoAsset.Insert(
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

		err = r.Video.Delete(t.Context(), video.ID)
		require.NoError(t, err)

		_, err = r.Video.Select(t.Context(), video.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)

		remainingAssets, err := r.VideoAsset.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.Empty(t, remainingAssets)

		var fileCount int
		row := bobDB.QueryRowContext(t.Context(), "SELECT count(*) FROM app.files WHERE file_id = $1", asset.FileID)
		require.NoError(t, row.Scan(&fileCount))
		require.Zero(t, fileCount, "file must not remain as orphan after video deletion")
	})
}

// TestRepository_VideoSelectQueuePositions_ComputesPositionAndTotalPerLane проверяет корректность
// позиции и размера полосы одним SQL-запросом с оконными функциями (§3 дизайна эпика Э5, В-3):
// архивная и срочная полосы нумеруются независимо (PARTITION BY is_urgent) в порядке
// status_changed_at, позиция считается глобально по системе — видео двух разных групп конкурируют
// за одну и ту же очередь, видео вне статуса queued в результат не попадают.
func TestRepository_VideoSelectQueuePositions_ComputesPositionAndTotalPerLane(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		accountRole, _ := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
		user, _ := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)

		// Две разные группы одного аккаунта — физическая очередь общесистемная, а не в рамках
		// группы (§3 дизайна эпика), поэтому видео разных групп должны конкурировать за одну и
		// ту же нумерацию внутри своей полосы.
		groupA, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		groupB, _ := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())

		insertQueued := func(groupID uuid.UUID, isUrgent bool, at time.Time) domain.Video {
			video, err := r.Video.Insert(
				t.Context(), f.Beer().Name(), groupID, user.ID, domain.VideoStatusQueued, isUrgent,
			)
			require.NoError(t, err)
			backdateVideoTimestamp(t, bobDB, video.ID, at)
			return video
		}

		base := time.Now().Add(-time.Hour)

		// Архивная полоса: 3 видео из разных групп по возрастанию времени постановки в очередь.
		archive1 := insertQueued(groupA.ID, false, base)
		archive2 := insertQueued(groupB.ID, false, base.Add(time.Minute))
		archive3 := insertQueued(groupA.ID, false, base.Add(2*time.Minute))

		// Срочная полоса: 2 видео — независимая нумерация, не пересекается с архивной.
		urgent1 := insertQueued(groupB.ID, true, base.Add(30*time.Second))
		urgent2 := insertQueued(groupA.ID, true, base.Add(90*time.Second))

		// Видео вне очереди — не должно попасть в результат вовсе.
		ready, err := r.Video.Insert(t.Context(), f.Beer().Name(), groupA.ID, user.ID, domain.VideoStatusReady, false)
		require.NoError(t, err)

		positions, err := r.Video.SelectQueuePositions(t.Context())
		require.NoError(t, err)

		require.Equal(t, domain.QueuePosition{Position: 1, Total: 3}, positions[archive1.ID])
		require.Equal(t, domain.QueuePosition{Position: 2, Total: 3}, positions[archive2.ID])
		require.Equal(t, domain.QueuePosition{Position: 3, Total: 3}, positions[archive3.ID])

		require.Equal(t, domain.QueuePosition{Position: 1, Total: 2}, positions[urgent1.ID])
		require.Equal(t, domain.QueuePosition{Position: 2, Total: 2}, positions[urgent2.ID])

		_, ok := positions[ready.ID]
		require.False(t, ok, "video outside queued status must not be present in the result")
	})
}

// TestRepository_VideoSelectQueuePositions_NoQueuedVideos_ReturnsEmptyMap проверяет, что при
// пустой очереди метод не падает и не путает отсутствие строк с ошибкой.
func TestRepository_VideoSelectQueuePositions_NoQueuedVideos_ReturnsEmptyMap(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		_ = newTestVideo(t, r, f, domain.VideoStatusReady)

		positions, err := r.Video.SelectQueuePositions(t.Context())

		require.NoError(t, err)
		require.Empty(t, positions)
	})
}
