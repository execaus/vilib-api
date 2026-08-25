package repository_test

import (
	"testing"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/dberrors"
	"vilib-api/internal/repository"
	"vilib-api/testutil"

	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
	"github.com/stephenafamo/bob"
	"github.com/stretchr/testify/require"
)

func TestRepository_UserGroupInsert_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		var (
			name = f.Beer().Name()
		)

		account, _ := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		group, err := r.UserGroup.Insert(t.Context(), account.ID, name)

		require.Nil(t, err)

		require.NotEmpty(t, group.ID)
		require.Equal(t, name, group.Name)
		require.Equal(t, account.ID, group.AccountID)

	})
}

func TestRepository_UserGroupSelectByID_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		const (
			accountCount   = 3
			userGroupCount = 5
		)

		generatedAccount := make([]domain.Account, accountCount)
		generatedUserGroupsID := make([][]uuid.UUID, userGroupCount)

		for i := range accountCount {
			generatedAccount[i], _ = r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)

			generatedUserGroupsID[i] = make([]uuid.UUID, userGroupCount)
			for j := range userGroupCount {
				userGroup, _ := r.UserGroup.Insert(t.Context(), generatedAccount[i].ID, f.Company().Name())
				generatedUserGroupsID[i][j] = userGroup.ID
			}
		}

		userGroups, err := r.UserGroup.GetByID(t.Context(), generatedUserGroupsID[2]...)

		require.Nil(t, err)
		require.Len(t, userGroups, userGroupCount)

		expectedIDs := generatedUserGroupsID[2]

		for _, role := range userGroups {
			require.Contains(t, expectedIDs, role.ID)
		}
	})
}

func TestRepository_UserGroupSelectByID_NilNotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		roles, err := r.UserGroup.GetByID(t.Context(), uuid.New())

		require.Nil(t, roles)
		require.ErrorIs(t, repository.ErrNotFound, err)
	})
}

// TestRepository_UserGroupDeleteCascade_RemovesGroupVideosAssetsFilesAndMembers проверяет
// каскадное удаление группы (Э1-Т21): видео, ассеты и файлы видео, участники и сама группа
// удаляются, а идентификаторы удалённых видео возвращаются вызывающей стороне для
// последующей best-effort зачистки объектов в хранилище.
func TestRepository_UserGroupDeleteCascade_RemovesGroupVideosAssetsFilesAndMembers(t *testing.T) {
	t.Parallel()

	testutil.WithDB(t, []string{"../../migrations"}, func(bobDB *bob.DB) {
		provider := repository.NewExecutorProvider(bobDB)
		r := repository.NewRepository(provider)
		f := testutil.Faker

		account, err := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		require.NoError(t, err)

		group, err := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		require.NoError(t, err)

		accountRole, err := r.AccountRole.Insert(t.Context(), account.ID, f.Beer().Name(), nil, 4, true, false)
		require.NoError(t, err)

		user, err := r.User.Insert(
			t.Context(),
			f.Person().FirstName(),
			f.Person().LastName(),
			f.Hash().MD5(),
			f.Person().Contact().Email,
			accountRole.ID,
		)
		require.NoError(t, err)

		video, err := r.Video.Insert(t.Context(), f.Beer().Name(), group.ID, user.ID, domain.VideoStatusReady, false)
		require.NoError(t, err)

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

		groupRole, err := r.GroupRole.Insert(t.Context(), account.ID, f.Beer().Name(), 1, true)
		require.NoError(t, err)

		_, err = r.GroupMember.Insert(t.Context(), group.ID, groupRole.ID, user.ID)
		require.NoError(t, err)

		deletedVideoIDs, err := r.UserGroup.DeleteCascade(t.Context(), group.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []uuid.UUID{video.ID}, deletedVideoIDs)

		_, err = r.Video.Select(t.Context(), video.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)

		remainingAssets, err := r.VideoAsset.Select(t.Context(), video.ID)
		require.NoError(t, err)
		require.Empty(t, remainingAssets)

		var fileCount int
		row := bobDB.QueryRowContext(t.Context(), "SELECT count(*) FROM app.files WHERE file_id = $1", asset.FileID)
		require.NoError(t, row.Scan(&fileCount))
		require.Zero(t, fileCount, "file must not remain as orphan after group deletion")

		_, err = r.UserGroup.GetByID(t.Context(), group.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)

		_, err = r.GroupMember.SelectByUserIDAndGroupID(t.Context(), user.ID, group.ID)
		require.Error(t, err)
	})
}

// TestRepository_UserGroupDeleteCascade_EmptyGroup_ReturnsNoIDs проверяет, что удаление
// группы без видео не падает и возвращает пустой список.
func TestRepository_UserGroupDeleteCascade_EmptyGroup_ReturnsNoIDs(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		require.NoError(t, err)

		group, err := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		require.NoError(t, err)

		deletedVideoIDs, err := r.UserGroup.DeleteCascade(t.Context(), group.ID)
		require.NoError(t, err)
		require.Empty(t, deletedVideoIDs)

		_, err = r.UserGroup.GetByID(t.Context(), group.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_UserGroupUpdateName_Success(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		require.NoError(t, err)

		group, err := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		require.NoError(t, err)

		newName := f.Beer().Name()

		updated, err := r.UserGroup.UpdateName(t.Context(), group.ID, newName)
		require.NoError(t, err)
		require.Equal(t, group.ID, updated.ID)
		require.Equal(t, newName, updated.Name)
		require.Equal(t, account.ID, updated.AccountID)

		groups, err := r.UserGroup.GetByID(t.Context(), group.ID)
		require.NoError(t, err)
		require.Equal(t, newName, groups[0].Name)
	})
}

func TestRepository_UserGroupUpdateName_NotFound(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		_, err := r.UserGroup.UpdateName(t.Context(), uuid.New(), f.Beer().Name())

		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestRepository_UserGroupUpdateName_DuplicateNameReturnsError(t *testing.T) {
	t.Parallel()

	testutil.TestRepositoryWithDB(t, func(r *repository.Repository, f faker.Faker) {
		account, err := r.Account.Insert(t.Context(), f.Company().Name(), f.Person().Contact().Email)
		require.NoError(t, err)

		existing, err := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		require.NoError(t, err)

		group, err := r.UserGroup.Insert(t.Context(), account.ID, f.Beer().Name())
		require.NoError(t, err)

		_, err = r.UserGroup.UpdateName(t.Context(), group.ID, existing.Name)

		require.ErrorIs(t, dberrors.UserGroupErrors.ErrUniqueUserGroupsNameAccountIdKey, err)
	})
}
