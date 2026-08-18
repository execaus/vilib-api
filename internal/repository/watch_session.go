package repository

import (
	"context"
	"errors"
	"time"
	"vilib-api/internal/domain"
	"vilib-api/internal/gen/schema"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"go.uber.org/zap"
)

// watchSessionMaxSeq — граница диапазона int32, в котором хранится last_seq сессии
// (schema.WatchSession.LastSeq). Клиент присылает seq как int64 (§3 контракта Э3), но
// в пределах одной сессии просмотра число heartbeat'ов заведомо не приближается к границе
// int32 — переполнение невозможно.
const watchSessionMaxSeq = int64(^uint32(0) >> 1)

// int32FromSeq конвертирует порядковый номер heartbeat'а в int32 для хранения в БД.
func int32FromSeq(seq int64) int32 {
	if seq > watchSessionMaxSeq {
		seq = watchSessionMaxSeq
	}
	return int32(seq) // #nosec G115 -- значение ограничено выше watchSessionMaxSeq
}

type WatchSessionRepository struct {
	provider *ExecutorProvider
}

func NewWatchSessionRepository(provider *ExecutorProvider) *WatchSessionRepository {
	return &WatchSessionRepository{provider: provider}
}

// SelectForUpdate выбирает и блокирует (FOR UPDATE) сессию по идентификатору.
func (r *WatchSessionRepository) SelectForUpdate(
	ctx context.Context, sessionID uuid.UUID,
) (domain.WatchSession, error) {
	exec := r.provider.GetExecutor(ctx)

	row, err := schema.WatchSessions.Query(
		sm.Where(schema.WatchSessions.Columns.SessionID.EQ(psql.Arg(sessionID))),
		sm.ForUpdate(),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.WatchSession{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.WatchSession{}, err
	}

	var session domain.WatchSession
	session.FromDB(row)

	return session, nil
}

// Insert создаёт новую сессию просмотра — session_id генерирует клиент при открытии плеера.
func (r *WatchSessionRepository) Insert(
	ctx context.Context, sessionID, userID, videoID uuid.UUID, now time.Time, positionMs int64,
) (domain.WatchSession, error) {
	exec := r.provider.GetExecutor(ctx)

	row, err := schema.WatchSessions.Insert(&schema.WatchSessionSetter{
		SessionID:      omit.From(sessionID),
		UserID:         omit.From(userID),
		VideoID:        omit.From(videoID),
		LastSeq:        omit.From(int32(0)),
		StartedAt:      omit.From(now),
		LastAt:         omit.From(now),
		LastPositionMS: omit.From(positionMs),
	}).One(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return domain.WatchSession{}, err
	}

	var session domain.WatchSession
	session.FromDB(row)

	return session, nil
}

// Update продвигает сессию на очередной принятый heartbeat: last_seq, last_at,
// last_position_ms.
func (r *WatchSessionRepository) Update(
	ctx context.Context, sessionID uuid.UUID, seq int64, now time.Time, positionMs int64,
) (domain.WatchSession, error) {
	exec := r.provider.GetExecutor(ctx)

	setter := &schema.WatchSessionSetter{
		LastSeq:        omit.From(int32FromSeq(seq)),
		LastAt:         omit.From(now),
		LastPositionMS: omit.From(positionMs),
	}

	row, err := schema.WatchSessions.Update(
		setter.UpdateMod(),
		um.Where(schema.WatchSessions.Columns.SessionID.EQ(psql.Arg(sessionID))),
	).One(ctx, exec)
	if err != nil {
		if errors.Is(pgx.ErrNoRows, err) {
			return domain.WatchSession{}, ErrNotFound
		}
		zap.L().Error(err.Error())
		return domain.WatchSession{}, err
	}

	var session domain.WatchSession
	session.FromDB(row)

	return session, nil
}

// DeleteOlderThan удаляет сессии, чей last_at старше cutoff — периодическая чистка watchdog'ом
// (В-52). Возвращает число удалённых строк.
func (r *WatchSessionRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	exec := r.provider.GetExecutor(ctx)

	rows, err := schema.WatchSessions.Delete(
		dm.Where(schema.WatchSessions.Columns.LastAt.LT(psql.Arg(cutoff))),
	).Exec(ctx, exec)
	if err != nil {
		zap.L().Error(err.Error())
		return 0, err
	}

	return rows, nil
}
