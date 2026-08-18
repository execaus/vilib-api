package dto

import (
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

// WatchHeartbeatRequest — тело heartbeat'а плеера: отрезок непрерывного воспроизведения с
// момента предыдущего принятого heartbeat'а этой сессии (§3 дизайна эпика Э3).
type WatchHeartbeatRequest struct {
	// SessionID — идентификатор сессии просмотра, генерируется клиентом при открытии плеера.
	SessionID uuid.UUID `json:"session_id" binding:"required"`
	// Seq — порядковый номер heartbeat'а в рамках сессии, обеспечивает идемпотентность повторов.
	Seq int64 `json:"seq" binding:"required,min=1"`
	// FromMs — начало присланного отрезка воспроизведения в миллисекундах.
	FromMs int64 `json:"from_ms" binding:"min=0"`
	// ToMs — конец присланного отрезка; ToMs == FromMs означает отсутствие воспроизведения
	// (пауза или перемотка) с прошлого heartbeat'а.
	ToMs int64 `json:"to_ms" binding:"min=0"`
	// PositionMs — текущая позиция плеера, используется для «продолжить с».
	PositionMs int64 `json:"position_ms" binding:"min=0"`
	// PlaybackRate — скорость воспроизведения; выше 1.0 отбрасывает интервал целиком (В-1
	// решение владельца).
	PlaybackRate float64 `json:"playback_rate" binding:"required,gt=0"`
	// ClientTS — время клиента на момент отправки, используется только в логе.
	ClientTS time.Time `json:"client_ts"`
}

// ToDomain конвертирует запрос heartbeat'а в domain.Heartbeat.
func (r *WatchHeartbeatRequest) ToDomain() domain.Heartbeat {
	return domain.Heartbeat{
		SessionID:    r.SessionID,
		Seq:          r.Seq,
		FromMs:       r.FromMs,
		ToMs:         r.ToMs,
		PositionMs:   r.PositionMs,
		PlaybackRate: r.PlaybackRate,
		ClientTS:     r.ClientTS,
	}
}

// WatchProgressResponse — состояние прогресса просмотра видео (§3 дизайна эпика Э3). Accepted
// значим только в ответе на heartbeat — зачтён ли присланный интервал (для отладки/e2e);
// в ответе GET .../progress всегда false.
type WatchProgressResponse struct {
	CoveredMs      int64  `json:"covered_ms"`
	CoveragePct    int    `json:"coverage_pct"`
	Completed      bool   `json:"completed"`
	LastPositionMs int64  `json:"last_position_ms"`
	DurationMs     *int64 `json:"duration_ms,omitempty"`
	Accepted       bool   `json:"accepted"`
}

// FromDomain заполняет WatchProgressResponse состоянием прогресса просмотра.
func (r *WatchProgressResponse) FromDomain(state domain.WatchState) {
	r.CoveredMs = state.CoveredMs
	r.CoveragePct = state.CoveragePct
	r.Completed = state.Completed
	r.LastPositionMs = state.LastPositionMs
	r.DurationMs = state.DurationMs
	r.Accepted = state.Accepted
}
