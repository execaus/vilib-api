package service

var (
	ErrForbidden = NewForbiddenError("user creation is forbidden")
)

// UnauthorizedError — ошибка отсутствующей или недействительной аутентификации (HTTP 401):
// например, просроченный или подделанный HLS-токен (§4.2 дизайна эпика).
type UnauthorizedError struct {
	message string
}

func NewUnauthorizedError(message string) *UnauthorizedError {
	return &UnauthorizedError{message: message}
}

func (e UnauthorizedError) Error() string {
	return e.message
}

// ErrUnauthorized — sentinel-ошибка недействительного HLS-токена (просрочен, битая подпись,
// неверный purpose).
var ErrUnauthorized = NewUnauthorizedError("unauthorized")

type ConflictError struct {
	message string
}

func NewConflictError(message string) ConflictError {
	return ConflictError{message: message}
}

func (e ConflictError) Error() string {
	return e.message
}

type ForbiddenError struct {
	message string
}

func NewForbiddenError(message string) *ForbiddenError {
	return &ForbiddenError{message: message}
}

func (e ForbiddenError) Error() string {
	return e.message
}

// ErrValidation — sentinel-ошибка валидации входных данных на уровне сервиса (HTTP 400).
var ErrValidation = NewValidationError("validation error")

// ValidationError — ошибка валидации входных данных, которую handler превращает в HTTP 400.
type ValidationError struct {
	message string
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{message: message}
}

func (e ValidationError) Error() string {
	return e.message
}
