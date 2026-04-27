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

type VideoRepository struct {
	provider *ExecutorProvider
}

func NewVideoRepository(provider *ExecutorProvider) *VideoRepository {
	return &VideoRepository{provider: provider}
}

func (r *VideoRepository) Select(ctx context.Context, id uuid.UUID) (*domain.Video, error) {
	exec := r.provider.GetExecutor(ctx)

	videoDB, err := schema.UserGroupVideos.Query(
		sm.Where(schema.UserGroupVideos.Columns.ID.EQ(psql.Arg(id))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return nil, ErrNotFound
		}
		zap.L().Error(err.Error())
		return nil, err
	}

	video := &domain.Video{}
	video.FromDB(videoDB)

	return video, nil
}

func (r *VideoRepository) Insert(
	ctx context.Context,
	name string,
	groupID, userID uuid.UUID,
	status domain.VideoStatus,
) (domain.Video, error) {
	exec := r.provider.GetExecutor(ctx)

	videoDB, err := schema.UserGroupVideos.Insert(&schema.UserGroupVideoSetter{
		Name:        omit.From(name),
		UserGroupID: omit.From(groupID),
		Author:      omit.From(userID),
		Status:      omit.From(int32(status)),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	var video domain.Video
	video.FromDB(videoDB)

	return video, nil
}

func (r *VideoRepository) Update(ctx context.Context, id uuid.UUID, status *domain.VideoStatus) (domain.Video, error) {
	exec := r.provider.GetExecutor(ctx)

	videoDB, err := schema.UserGroupVideos.Query(
		sm.Where(schema.UserGroupVideos.Columns.ID.EQ(psql.Arg(id))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Video{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	err = videoDB.Update(ctx, exec, &schema.UserGroupVideoSetter{
		Status: omit.From(int32(*status)),
	})
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	var video domain.Video
	video.FromDB(videoDB)

	return video, nil
}

func (r *VideoRepository) SelectByGroupID(ctx context.Context, groupID uuid.UUID) ([]domain.Video, error) {
	exec := r.provider.GetExecutor(ctx)

	videosDB, err := schema.UserGroupVideos.Query(
		sm.Where(schema.UserGroupVideos.Columns.UserGroupID.EQ(psql.Arg(groupID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	videos := make([]domain.Video, len(videosDB))
	for i, v := range videosDB {
		videos[i] = domain.Video{}
		videos[i].FromDB(v)
	}

	return videos, nil
}

func (r *VideoRepository) UpdateName(ctx context.Context, videoID uuid.UUID, name string) (domain.Video, error) {
	exec := r.provider.GetExecutor(ctx)

	videoDB, err := schema.UserGroupVideos.Query(
		sm.Where(schema.UserGroupVideos.Columns.ID.EQ(psql.Arg(videoID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.Video{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	err = videoDB.Update(ctx, exec, &schema.UserGroupVideoSetter{
		Name: omit.From(name),
	})
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	var video domain.Video
	video.FromDB(videoDB)

	return video, nil
}

func (r *VideoRepository) Delete(ctx context.Context, videoID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	_, err := schema.UserGroupVideos.Delete(
		dm.Where(schema.UserGroupVideos.Columns.ID.EQ(psql.Arg(videoID))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
