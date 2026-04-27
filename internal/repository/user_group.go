package repository

import (
	"context"
	"errors"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
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

func (r *UserGroupRepository) DeleteCascade(ctx context.Context, groupID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	// 1. Получить список видео группы
	videosDB, err := schema.UserGroupVideos.Query(
		sm.Where(schema.UserGroupVideos.Columns.UserGroupID.EQ(psql.Arg(groupID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// 2. Удалить video_assets и files для каждого видео
	for _, video := range videosDB {
		assetsDB, err := schema.VideoAssets.Query(
			sm.Where(schema.VideoAssets.Columns.VideoID.EQ(psql.Arg(video.ID))),
		).All(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}

		for _, asset := range assetsDB {
			_, err = schema.Files.Delete(
				dm.Where(schema.Files.Columns.FileID.EQ(psql.Arg(asset.FileID))),
			).Exec(ctx, exec)
			if err != nil {
				zap.L().Error(err.Error())
				return err
			}
		}

		_, err = schema.VideoAssets.Delete(
			dm.Where(schema.VideoAssets.Columns.VideoID.EQ(psql.Arg(video.ID))),
		).Exec(ctx, exec)
		if err != nil {
			zap.L().Error(err.Error())
			return err
		}
	}

	// 3. Удалить user_group_videos
	_, err = schema.UserGroupVideos.Delete(
		dm.Where(schema.UserGroupVideos.Columns.UserGroupID.EQ(psql.Arg(groupID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// 4. Удалить group_members
	_, err = schema.GroupMembers.Delete(
		dm.Where(schema.GroupMembers.Columns.GroupID.EQ(psql.Arg(groupID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	// 5. Удалить user_group
	_, err = schema.UserGroups.Delete(
		dm.Where(schema.UserGroups.Columns.GroupID.EQ(psql.Arg(groupID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
