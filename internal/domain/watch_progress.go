package domain

import (
	"time"
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

// WatchProgress — прогресс просмотра видео пользователем: объединённые интервалы heartbeat'ов
// и накопленные метрики (§1.4 дизайна эпика Э3). Intervals хранит текстовое представление
// `int8multirange` (например, `{[0,10000),[15000,30000)}`) — объединение и подсчёт покрытия
// делает PostgreSQL, в Go интервалы не парсятся (§0 дизайна эпика: bobgen переводит
// неизвестные типы столбцов в string).
type WatchProgress struct {
	UserID             uuid.UUID
	VideoID            uuid.UUID
	Intervals          string
	CoveredMs          int64
	LastPositionMs     int64
	WallMs             int64
	FirstAt            time.Time
	LastAt             time.Time
	ThresholdReachedAt *time.Time
}

// FromDB заполняет WatchProgress строкой сгенерированной модели schema.WatchProgress.
func (p *WatchProgress) FromDB(db *schema.WatchProgress) {
	p.UserID = db.UserID
	p.VideoID = db.VideoID
	p.Intervals = db.Intervals
	p.CoveredMs = db.CoveredMS
	p.LastPositionMs = db.LastPositionMS
	p.WallMs = db.WallMS
	p.FirstAt = db.FirstAt
	p.LastAt = db.LastAt
	p.ThresholdReachedAt = dbconv.NullValToPtr(db.ThresholdReachedAt)
}

// WatchSession — сессия просмотра: идемпотентность heartbeat'ов и защита от перемотки в
// рамках одной сессии плеера (§1.5 дизайна эпика Э3).
type WatchSession struct {
	SessionID      uuid.UUID
	UserID         uuid.UUID
	VideoID        uuid.UUID
	LastSeq        int64
	StartedAt      time.Time
	LastAt         time.Time
	LastPositionMs int64
}

// FromDB заполняет WatchSession строкой сгенерированной модели schema.WatchSession.
func (s *WatchSession) FromDB(db *schema.WatchSession) {
	s.SessionID = db.SessionID
	s.UserID = db.UserID
	s.VideoID = db.VideoID
	s.LastSeq = int64(db.LastSeq)
	s.StartedAt = db.StartedAt
	s.LastAt = db.LastAt
	s.LastPositionMs = db.LastPositionMS
}

// Heartbeat — один принятый клиентом отрезок непрерывного воспроизведения с момента
// предыдущего heartbeat'а этой сессии (§3 дизайна эпика Э3, вход WatchProgressService.Heartbeat).
type Heartbeat struct {
	SessionID    uuid.UUID
	Seq          int64
	FromMs       int64
	ToMs         int64
	PositionMs   int64
	PlaybackRate float64
	ClientTS     time.Time
}

// WatchState — состояние прогресса просмотра после обработки heartbeat'а или по запросу
// GET .../progress (§3 дизайна эпика Э3). CoveragePct — 0, если длительность видео ещё
// неизвестна; Accepted — был ли зачтён присланный интервал (только у ответа на heartbeat).
type WatchState struct {
	CoveredMs      int64
	CoveragePct    int
	Completed      bool
	LastPositionMs int64
	DurationMs     *int64
	Accepted       bool
}
