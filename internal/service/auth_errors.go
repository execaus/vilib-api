package service

import "errors"

var (
	ErrTokenInvalid = NewConflictError("invalid token")
)

var (
	ErrAccountsNotFound = errors.New("accounts not found")
)
