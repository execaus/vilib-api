package service

import "errors"

var (
	ErrTokenInvalid = NewServiceError("invalid token")
)

var (
	ErrAccountsNotFound = errors.New("accounts not found")
)
