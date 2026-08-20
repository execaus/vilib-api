package repository

import "errors"

var (
	ErrNotFound = errors.New("not found")
	// ErrInvalidDueMode — режим срока назначения без соответствующего значения (date без
	// due_at, days без due_days). Сигнализирует об ошибке вызывающего кода: валидацию
	// выполняет сервис до обращения к репозиторию.
	ErrInvalidDueMode = errors.New("invalid due mode")
)
