package saga

import (
	"context"

	"go.uber.org/zap"
)

// HooksCtxKey — ключ контекста, по которому Run кладёт *Hooks текущей саги.
const HooksCtxKey ctxKey = "saga-hooks-key"

// Hooks — очередь best-effort действий, выполняемых после успешного коммита транзакции саги
// (§7.3 эпика). Ошибки хуков не влияют на исход транзакции — она уже завершена.
type Hooks struct {
	fns []func(ctx context.Context)
}

// AfterCommit регистрирует fn для выполнения после успешного Commit транзакции саги, в рамках
// которой выполняется ctx. Хук получает исходный (нетранзакционный) контекст — обращение к
// репозиториям внутри хука работает напрямую с БД, а не с уже закрытой транзакцией. Хук не
// возвращает ошибку — обязан залогировать её самостоятельно. Вызов вне saga.Run — no-op с
// предупреждением в лог.
func AfterCommit(ctx context.Context, fn func(ctx context.Context)) {
	hooks, ok := ctx.Value(HooksCtxKey).(*Hooks)
	if !ok {
		zap.L().Warn("saga.AfterCommit called outside of saga.Run, hook is discarded")
		return
	}

	hooks.fns = append(hooks.fns, fn)
}

// run последовательно выполняет зарегистрированные хуки с ctx. Паника в хуке перехватывается
// и логируется, не прерывая выполнение остальных хуков.
func (h *Hooks) run(ctx context.Context) {
	for _, fn := range h.fns {
		runHookSafely(ctx, fn)
	}
}

// runHookSafely оборачивает вызов хука восстановлением после паники — сбой одного
// best-effort действия не должен ронять релей/сагу.
func runHookSafely(ctx context.Context, fn func(ctx context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			zap.L().Error("panic in saga after-commit hook", zap.Any("recover", r))
		}
	}()

	fn(ctx)
}
