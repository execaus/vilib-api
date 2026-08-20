// Package watchdog содержит фоновый страховочный процесс API (§8 дизайна эпика, Э1-Т16):
// периодически переводит в failed видео, зависшие в uploading/queued/compressing дольше
// сконфигурированных таймаутов.
package watchdog

import (
	"context"
	"time"
	"vilib-api/config"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"go.uber.org/zap"
)

// Watchdog — фоновый процесс с тикером cfg.WatchdogInterval. Каждый тик выполняется в
// отдельной транзакции саги через тот же runner, что и остальная бизнес-логика — так
// побочные эффекты после коммита (best-effort очистка S3) работают единообразно.
type Watchdog struct {
	runner   saga.Runner[*service.Service]
	interval time.Duration
	logger   *zap.Logger
}

// New создаёт watchdog с периодом тика cfg.WatchdogInterval.
func New(runner saga.Runner[*service.Service], cfg config.VideoConfig, logger *zap.Logger) *Watchdog {
	return &Watchdog{runner: runner, interval: cfg.WatchdogInterval, logger: logger}
}

// Run блокируется до отмены ctx: первый тик выполняется немедленно при старте, затем — с
// периодом interval. Ошибка тика логируется, но не останавливает watchdog — единичный сбой
// (например, временная недоступность БД) не должен отключать страховочный процесс до
// следующего тика.
func (w *Watchdog) Run(ctx context.Context) error {
	w.tick(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick выполняет один проход watchdog'а внутри транзакции саги: три условных UPDATE по
// таймаутам uploading/queued/compressing (VideoService.FailTimedOut), удаление устаревших
// сессий просмотра (решение О-2 эпика Э3) и best-effort очистка объектов зависших загрузок
// после коммита. Несколько инстансов API безопасны без
// дополнительной синхронизации между собой: каждый UPDATE атомарен и условен, поэтому строку,
// уже переведённую другим инстансом на этом же тике, повторный UPDATE просто не затронет
// (WHERE status = <исходный статус> перестаёт совпадать после первого перевода).
func (w *Watchdog) tick(ctx context.Context) {
	now := time.Now()

	err := w.runner.Run(ctx, func(txCtx context.Context, s *service.Service) error {
		if _, failErr := s.Video.FailTimedOut(txCtx, now); failErr != nil {
			return failErr
		}

		// Чистка сессий просмотра старше срока хранения (решение О-2 эпика Э3): отдельного
		// фонового процесса ради одного DELETE не заводим — тик watchdog'а уже идёт с нужной
		// периодичностью и в транзакции саги.
		deleted, cleanupErr := s.WatchProgress.CleanupStaleSessions(txCtx, now)
		if cleanupErr != nil {
			return cleanupErr
		}
		if deleted > 0 {
			w.logger.Info("watch sessions cleaned up", zap.Int64("deleted", deleted))
		}

		return nil
	})
	if err != nil {
		w.logger.Error("watchdog tick failed", zap.Error(err))
	}
}
