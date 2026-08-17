package service

// Коды ошибок по умолчанию для каждого типа (используются, когда вызывающий код не указал
// более специфичный код через конструктор XxxErrorCode) — §6.8 ТЗ.
const (
	codeUnauthorized = "unauthorized"
	codeConflict     = "conflict"
	codeForbidden    = "forbidden"
	codeValidation   = "validation"
)

var (
	ErrForbidden = NewForbiddenError("user creation is forbidden")
)

// UnauthorizedError — ошибка отсутствующей или недействительной аутентификации (HTTP 401):
// например, просроченный или подделанный HLS-токен (§4.2 дизайна эпика), просроченный или
// битый JWT (§2.1 дизайна эпика).
type UnauthorizedError struct {
	code    string
	message string
}

// NewUnauthorizedError создаёт UnauthorizedError с кодом по умолчанию ("unauthorized").
func NewUnauthorizedError(message string) *UnauthorizedError {
	return NewUnauthorizedErrorCode(codeUnauthorized, message)
}

// NewUnauthorizedErrorCode создаёт UnauthorizedError с явным машинным кодом.
func NewUnauthorizedErrorCode(code, message string) *UnauthorizedError {
	return &UnauthorizedError{code: code, message: message}
}

func (e UnauthorizedError) Error() string {
	return e.message
}

// Code возвращает машинный код ошибки для тела HTTP-ответа.
func (e UnauthorizedError) Code() string {
	return e.code
}

// ErrUnauthorized — sentinel-ошибка недействительного HLS-токена (просрочен, битая подпись,
// неверный purpose).
var ErrUnauthorized = NewUnauthorizedError("unauthorized")

// ConflictError — ошибка конфликта состояния (HTTP 409).
type ConflictError struct {
	code    string
	message string
}

// NewConflictError создаёт ConflictError с кодом по умолчанию ("conflict").
func NewConflictError(message string) ConflictError {
	return NewConflictErrorCode(codeConflict, message)
}

// NewConflictErrorCode создаёт ConflictError с явным машинным кодом.
func NewConflictErrorCode(code, message string) ConflictError {
	return ConflictError{code: code, message: message}
}

func (e ConflictError) Error() string {
	return e.message
}

// Code возвращает машинный код ошибки для тела HTTP-ответа.
func (e ConflictError) Code() string {
	return e.code
}

// ForbiddenError — ошибка недостатка прав (HTTP 403).
type ForbiddenError struct {
	code    string
	message string
}

// NewForbiddenError создаёт ForbiddenError с кодом по умолчанию ("forbidden").
func NewForbiddenError(message string) *ForbiddenError {
	return NewForbiddenErrorCode(codeForbidden, message)
}

// NewForbiddenErrorCode создаёт ForbiddenError с явным машинным кодом.
func NewForbiddenErrorCode(code, message string) *ForbiddenError {
	return &ForbiddenError{code: code, message: message}
}

func (e ForbiddenError) Error() string {
	return e.message
}

// Code возвращает машинный код ошибки для тела HTTP-ответа.
func (e ForbiddenError) Code() string {
	return e.code
}

// ErrValidation — sentinel-ошибка валидации входных данных на уровне сервиса (HTTP 400).
var ErrValidation = NewValidationError("validation error")

// ValidationError — ошибка валидации входных данных, которую handler превращает в HTTP 400.
type ValidationError struct {
	code    string
	message string
}

// NewValidationError создаёт ValidationError с кодом по умолчанию ("validation").
func NewValidationError(message string) *ValidationError {
	return NewValidationErrorCode(codeValidation, message)
}

// NewValidationErrorCode создаёт ValidationError с явным машинным кодом.
func NewValidationErrorCode(code, message string) *ValidationError {
	return &ValidationError{code: code, message: message}
}

func (e ValidationError) Error() string {
	return e.message
}

// Code возвращает машинный код ошибки для тела HTTP-ответа.
func (e ValidationError) Code() string {
	return e.code
}
