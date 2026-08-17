package repository

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

type UserGroupRepository struct {
	provider *ExecutorProvider
}

func NewUserGroupRepository(provider *ExecutorProvider) *UserGroupRepository {
	return &UserGroupRepository{provider: provider}
}

func (r *UserGroupRepository) Insert(
	ctx context.Context,
	accountID uuid.UUID,
	name string,
) (domain.UserGroup, error) {
	exec := r.provider.GetExecutor(ctx)

	userGroupDB, err := schema.UserGroups.Insert(&schema.UserGroupSetter{
		Name:      omit.From(name),
		AccountID: omit.From(accountID),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	userGroup := domain.UserGroup{}
	userGroup.FromDB(userGroupDB)

	return userGroup, nil
}

func (r *UserGroupRepository) GetByID(ctx context.Context, groupsID ...uuid.UUID) ([]domain.UserGroup, error) {
	exec := r.provider.GetExecutor(ctx)

	userGroups := make([]domain.UserGroup, len(groupsID))

	for i, id := range groupsID {
		userGroupDB, err := schema.UserGroups.Query(
			sm.Where(schema.UserGroups.Columns.GroupID.EQ(psql.Arg(id))),
		).One(ctx, exec)
		if err != nil {
			if errors.Is(pgx.ErrNoRows, err) {
				return nil, ErrNotFound
			}
			zap.L().Error(err.Error())
			return nil, err
		}

		userGroups[i] = domain.UserGroup{}
		userGroups[i].FromDB(userGroupDB)
	}

	return userGroups, nil
}

// UpdateName переименовывает группу (§4 дизайна эпика Э2, «Блок C»). Группа не найдена —
// ErrNotFound; дубль имени в пределах аккаунта — dberrors.UserGroupErrors.
// ErrUniqueUserGroupsNameAccountIdKey (проверяется вызывающим сервисом).
func (r *UserGroupRepository) UpdateName(
	ctx context.Context,
	groupID uuid.UUID,
	name string,
) (domain.UserGroup, error) {
	exec := r.provider.GetExecutor(ctx)

	userGroupDB, err := schema.UserGroups.Update(
		(&schema.UserGroupSetter{Name: omit.From(name)}).UpdateMod(),
		um.Where(schema.UserGroups.Columns.GroupID.EQ(psql.Arg(groupID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.UserGroup{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.UserGroup{}, err
	}

	userGroup := domain.UserGroup{}
	userGroup.FromDB(userGroupDB)

	return userGroup, nil
}

func (r *UserGroupRepository) SelectByAccountID(
	ctx context.Context,
	accountID uuid.UUID,
) ([]domain.UserGroup, error) {
	exec := r.provider.GetExecutor(ctx)

	groupsDB, err := schema.UserGroups.Query(
		sm.Where(schema.UserGroups.Columns.AccountID.EQ(psql.Arg(accountID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	groups := make([]domain.UserGroup, len(groupsDB))
	for i, g := range groupsDB {
		groups[i] = domain.UserGroup{}
		groups[i].FromDB(g)
	}

	return groups, nil
}

// DeleteCascade удаляет группу вместе со всеми её видео, ассетами, файлами и участниками
// (Э1-Т21). Порядок: сначала files, на которые ссылаются video_assets видео группы (каскадно
// убирает и сами video_assets — FK video_assets.file_id → files.file_id объявлен ON DELETE
// CASCADE), затем видео (FK video_assets.video_id → user_group_videos.id тоже ON DELETE
// CASCADE, но явное удаление files обязательно — иначе они остались бы сиротами в БД), затем
// участники и сама группа. Возвращает id удалённых видео — вызывающая сторона использует их
// для best-effort зачистки объектов в хранилище после коммита (§7.3 эпика).
func (r *UserGroupRepository) DeleteCascade(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	// 1. Получить список видео группы — id нужны вызывающей стороне для очистки S3.
	videosDB, err := schema.UserGroupVideos.Query(
		sm.Where(schema.UserGroupVideos.Columns.UserGroupID.EQ(psql.Arg(groupID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	videoIDs := make([]uuid.UUID, len(videosDB))
	for i, video := range videosDB {
		videoIDs[i] = video.ID
	}

	// 2. Удалить files, на которые ссылаются video_assets видео группы (одним запросом через
	// подзапрос по video_id) — каскадно убирает и сами строки video_assets.
	if len(videoIDs) > 0 {
		videoIDArgs := make([]bob.Expression, len(videoIDs))
		for i, id := range videoIDs {
			videoIDArgs[i] = psql.Arg(id)
		}

		assetsDB, assetsErr := schema.VideoAssets.Query(
			sm.Where(schema.VideoAssets.Columns.VideoID.In(videoIDArgs...)),
		).All(ctx, exec)
		if assetsErr != nil {
			zap.L().Error(assetsErr.Error())
			return nil, assetsErr
		}

		if len(assetsDB) > 0 {
			fileIDArgs := make([]bob.Expression, len(assetsDB))
			for i, asset := range assetsDB {
				fileIDArgs[i] = psql.Arg(asset.FileID)
			}

			if _, deleteErr := schema.Files.Delete(
				dm.Where(schema.Files.Columns.FileID.In(fileIDArgs...)),
			).Exec(ctx, exec); deleteErr != nil {
				zap.L().Error(deleteErr.Error())
				return nil, deleteErr
			}
		}
	}

	// 3. Удалить user_group_videos
	if _, err = schema.UserGroupVideos.Delete(
		dm.Where(schema.UserGroupVideos.Columns.UserGroupID.EQ(psql.Arg(groupID))),
	).Exec(ctx, exec); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// 4. Удалить group_members
	if _, err = schema.GroupMembers.Delete(
		dm.Where(schema.GroupMembers.Columns.GroupID.EQ(psql.Arg(groupID))),
	).Exec(ctx, exec); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	// 5. Удалить user_group
	if _, err = schema.UserGroups.Delete(
		dm.Where(schema.UserGroups.Columns.GroupID.EQ(psql.Arg(groupID))),
	).Exec(ctx, exec); err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	return videoIDs, nil
}
