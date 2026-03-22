package service

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
