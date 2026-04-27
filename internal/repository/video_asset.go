package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"go.uber.org/zap"
)

type VideoAssetRepository struct {
	provider *ExecutorProvider
}

func NewVideoAssetRepository(provider *ExecutorProvider) *VideoAssetRepository {
	return &VideoAssetRepository{provider: provider}
}

func (r *VideoAssetRepository) Select(ctx context.Context, videoID uuid.UUID) ([]domain.VideoAsset, error) {
	exec := r.provider.GetExecutor(ctx)

	assetsDB, err := schema.VideoAssets.Query(
		sm.Where(schema.VideoAssets.Columns.VideoID.EQ(psql.Arg(videoID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	assets := make([]domain.VideoAsset, len(assetsDB))
	for i, asset := range assetsDB {
		assets[i].FromDB(asset)
	}

	return assets, nil
}

func (r *VideoAssetRepository) Create(
	ctx context.Context,
	videoID uuid.UUID,
	tag domain.VideoAssetTag,
	bucketName, contentType string,
	bytes int,
) (domain.VideoAsset, error) {
	exec := r.provider.GetExecutor(ctx)

	fileDB, err := schema.Files.Insert(&schema.FileSetter{
		Bucket:      omit.From(bucketName),
		ContentType: omit.From(contentType),
		SizeBytes:   omit.From(int64(bytes)),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAsset{}, err
	}

	assetDB, err := schema.VideoAssets.Insert(&schema.VideoAssetSetter{
		FileID:  omit.From(fileDB.FileID),
		VideoID: omit.From(videoID),
		Tag:     omit.From(int32(tag)),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAsset{}, err
	}

	var asset domain.VideoAsset
	asset.FromDB(assetDB)

	return asset, nil
}
