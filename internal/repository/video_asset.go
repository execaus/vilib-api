package repository

import (
	"context"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
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
		fileDB, fileErr := schema.Files.Query(
			sm.Where(schema.Files.Columns.FileID.EQ(psql.Arg(asset.FileID))),
		).One(ctx, exec)
		if fileErr != nil {
			zap.L().Error(fileErr.Error())
			return nil, fileErr
		}

		assets[i].FromDBWithFile(asset, fileDB)
	}

	return assets, nil
}

// Insert регистрирует ассет видео: создаёт запись файла и связывает её с видео одной
// операцией. Уникальность (video_id, kind, profile) обеспечивается ограничением БД —
// повторная регистрация того же ассета возвращает ошибку нарушения уникальности.
func (r *VideoAssetRepository) Insert(
	ctx context.Context,
	videoID uuid.UUID,
	kind domain.VideoAssetKind,
	profile domain.VideoProfile,
	bucket, key, contentType string,
	sizeBytes int64,
) (domain.VideoAsset, error) {
	exec := r.provider.GetExecutor(ctx)

	fileDB, err := schema.Files.Insert(&schema.FileSetter{
		Bucket:      omit.From(bucket),
		ObjectKey:   omit.From(key),
		ContentType: omit.From(contentType),
		SizeBytes:   omit.From(sizeBytes),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAsset{}, err
	}

	assetDB, err := schema.VideoAssets.Insert(&schema.VideoAssetSetter{
		FileID:  omit.From(fileDB.FileID),
		VideoID: omit.From(videoID),
		Kind:    omit.From(string(kind)),
		Profile: omit.From(string(profile)),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.VideoAsset{}, err
	}

	var asset domain.VideoAsset
	asset.FromDBWithFile(assetDB, fileDB)

	return asset, nil
}

// DeleteByVideoAndKinds удаляет ассеты видео указанных видов вместе со связанными файлами
// (идемпотентная перерегистрация результатов обработки, Э1-Т14). Удаление выполняется через
// files: video_assets.file_id → files.file_id объявлен ON DELETE CASCADE, поэтому строки
// video_assets удаляются автоматически вслед за files. Отсутствие ассетов указанных видов —
// не ошибка.
func (r *VideoAssetRepository) DeleteByVideoAndKinds(
	ctx context.Context,
	videoID uuid.UUID,
	kinds []domain.VideoAssetKind,
) error {
	if len(kinds) == 0 {
		return nil
	}

	exec := r.provider.GetExecutor(ctx)

	kindArgs := make([]bob.Expression, len(kinds))
	for i, kind := range kinds {
		kindArgs[i] = psql.Arg(string(kind))
	}

	assetsDB, err := schema.VideoAssets.Query(
		sm.Where(schema.VideoAssets.Columns.VideoID.EQ(psql.Arg(videoID))),
		sm.Where(schema.VideoAssets.Columns.Kind.In(kindArgs...)),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
	}

	if len(assetsDB) == 0 {
		return nil
	}

	fileIDArgs := make([]bob.Expression, len(assetsDB))
	for i, asset := range assetsDB {
		fileIDArgs[i] = psql.Arg(asset.FileID)
	}

	if _, deleteErr := schema.Files.Delete(
		dm.Where(schema.Files.Columns.FileID.In(fileIDArgs...)),
	).Exec(ctx, exec); deleteErr != nil {
		zap.L().Error(deleteErr.Error())
		return deleteErr
	}

	return nil
}
