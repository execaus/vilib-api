package service

type ConflictError struct {
	message string
}

func NewServiceError(message string) ConflictError {
	return ConflictError{message: message}
}

func (e ConflictError) Error() string {
	return e.message
}
