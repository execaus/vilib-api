package service

var (
	ErrEmailInvalid      = NewServiceError("invalid email")
	ErrAccountNameExists = NewServiceError("account name exists")
)
