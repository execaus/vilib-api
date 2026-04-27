package handler

import "errors"

const (
	ErrCodeUserDeactivated = "user deactivated"
	ErrCodeNotFound        = "not found"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
)
