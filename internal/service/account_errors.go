package service

var (
	ErrEmailInvalid  = NewServiceError("invalid email")
	ErrAccountExists = NewServiceError("account exists")
)
