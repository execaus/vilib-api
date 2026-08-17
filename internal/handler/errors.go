package handler

import "errors"

// Машинные коды и тексты ошибок транспортного уровня (§6.8 ТЗ).
const (
	codeNotFound           = "not_found"
	codeValidation         = "validation"
	codeUnauthorized       = "unauthorized"
	codeUserDeactivated    = "user_deactivated"
	messageNotFound        = "not found"
	messageUserDeactivated = "user deactivated"
)

// ErrClaimsContextEmpty — claims отсутствуют в контексте запроса: программная ошибка,
// обработчик защищён RequireAuthMiddleware, но был вызван для маршрута без него.
var ErrClaimsContextEmpty = errors.New("claims context empty")
