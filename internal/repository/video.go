package repository

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
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

// int32FromVideoStatus конвертирует статус видео в int32 для хранения в БД. Набор статусов
// фиксирован и мал, поэтому переполнение невозможно.
func int32FromVideoStatus(status domain.VideoStatus) int32 {
	return int32(status) // #nosec G115 -- набор статусов видео фиксирован и заведомо мал
}

// int32FromInt конвертирует небольшое целое (номер попытки обработки, ширина/высота кадра)
// в int32 для хранения в БД. Значения контролируются приложением и никогда не приближаются
// к границам int32.
func int32FromInt(v int) int32 {
	return int32(v) // #nosec G115 -- значение контролируется приложением и заведомо мало
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
		Status:      omit.From(int32FromVideoStatus(status)),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.Video{}, err
	}

	var video domain.Video
	video.FromDB(videoDB)

	return video, nil
}

// UpdateStatusIf выполняет условный переход статуса: UPDATE применяется только к строке
// с id, чей текущий статус входит в from (и, если задан patch.ExpectedAttempt, у которой
// совпадает processing_attempt). Возвращает true, если строка была обновлена.
func (r *VideoRepository) UpdateStatusIf(
	ctx context.Context,
	id uuid.UUID,
	from []domain.VideoStatus,
	to domain.VideoStatus,
	patch domain.VideoPatch,
) (bool, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.UserGroupVideoSetter{
		Status:          omit.From(int32FromVideoStatus(to)),
		StatusChangedAt: omit.From(time.Now()),
	}

	if patch.ProcessingAttempt != nil {
		setter.ProcessingAttempt = omit.From(int32FromInt(*patch.ProcessingAttempt))
	}
	if patch.ClearFailure {
		setter.FailureClass = omitnull.FromPtr[string](nil)
		setter.FailureReason = omitnull.FromPtr[string](nil)
	}
	if patch.FailureClass != nil {
		setter.FailureClass = omitnull.From(string(*patch.FailureClass))
	}
	if patch.FailureReason != nil {
		setter.FailureReason = omitnull.From(*patch.FailureReason)
	}
	if patch.DurationMs != nil {
		setter.DurationMS = omitnull.From(*patch.DurationMs)
	}
	if patch.Width != nil {
		setter.Width = omitnull.From(int32FromInt(*patch.Width))
	}
	if patch.Height != nil {
		setter.Height = omitnull.From(int32FromInt(*patch.Height))
	}

	fromArgs := make([]bob.Expression, len(from))
	for i, status := range from {
		fromArgs[i] = psql.Arg(int32FromVideoStatus(status))
	}

	mods := []bob.Mod[*dialect.UpdateQuery]{
		setter.UpdateMod(),
		um.Where(schema.UserGroupVideos.Columns.ID.EQ(psql.Arg(id))),
		um.Where(schema.UserGroupVideos.Columns.Status.In(fromArgs...)),
	}
	if patch.ExpectedAttempt != nil {
		mods = append(
			mods,
			um.Where(
				schema.UserGroupVideos.Columns.ProcessingAttempt.EQ(psql.Arg(int32FromInt(*patch.ExpectedAttempt))),
			),
		)
	}

	affected, err := schema.UserGroupVideos.Update(mods...).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return false, err
	}

	return affected > 0, nil
}

// UpdateTimedOut переводит в failed(watchdog timeout) все видео заданного статуса, у которых
// контрольная временная метка старше before — один атомарный условный UPDATE (§8 дизайна
// эпика). Контрольная метка выбирается по статусу: created_at для uploading (время начала
// загрузки), status_changed_at для остальных статусов (время входа в текущий статус). Один
// SQL-запрос атомарен, поэтому несколько инстансов API безопасны без дополнительной
// блокировки: строка обновляется ровно одним инстансом, остальные получат её уже в статусе
// failed и не попадут под условие WHERE status = $1.
func (r *VideoRepository) UpdateTimedOut(
	ctx context.Context,
	status domain.VideoStatus,
	before time.Time,
	failure domain.VideoFailure,
) ([]uuid.UUID, error) {
	exec := r.provider.GetExecutor(ctx)

	timestampColumn := schema.UserGroupVideos.Columns.StatusChangedAt
	if status == domain.VideoStatusUploading {
		timestampColumn = schema.UserGroupVideos.Columns.CreatedAt
	}

	setter := &schema.UserGroupVideoSetter{
		Status:          omit.From(int32FromVideoStatus(domain.VideoStatusFailed)),
		StatusChangedAt: omit.From(time.Now()),
		FailureClass:    omitnull.From(string(failure.Class)),
		FailureReason:   omitnull.From(failure.Reason),
	}

	rows, err := schema.UserGroupVideos.Update(
		setter.UpdateMod(),
		um.Where(schema.UserGroupVideos.Columns.Status.EQ(psql.Arg(int32FromVideoStatus(status)))),
		um.Where(timestampColumn.LT(psql.Arg(before))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return nil, err
	}

	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}

	return ids, nil
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

// Delete удаляет видео вместе со всеми его ассетами и файлами (Э1-Т21): сначала удаляются
// строки files, на которые ссылаются video_assets видео (это каскадно убирает и сами строки
// video_assets — video_assets.file_id → files.file_id объявлен ON DELETE CASCADE), затем
// строка user_group_videos. Явное удаление files обязательно: FK video_assets.video_id →
// user_group_videos.id тоже ON DELETE CASCADE, но он чистит только video_assets — без явного
// шага строки files остались бы в БД сиротами, что недопустимо (Э1-Т21: сирота в S3 допустима,
// сирота в БД — нет).
func (r *VideoRepository) Delete(ctx context.Context, videoID uuid.UUID) error {
	exec := r.provider.GetExecutor(ctx)

	assetsDB, err := schema.VideoAssets.Query(
		sm.Where(schema.VideoAssets.Columns.VideoID.EQ(psql.Arg(videoID))),
	).All(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return err
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
			return deleteErr
		}
	}

	if _, err = schema.UserGroupVideos.Delete(
		dm.Where(schema.UserGroupVideos.Columns.ID.EQ(psql.Arg(videoID))),
	).Exec(ctx, exec); err != nil {
		zap.L().Error(err.Error())
		return err
	}

	return nil
}
