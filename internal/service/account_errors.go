package service

var (
	ErrEmailInvalid      = NewConflictError("invalid email")
	ErrAccountNameExists = NewConflictError("account name exists")
)
