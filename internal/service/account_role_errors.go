package service

import "errors"

var (
	ErrDefaultRoleNotFound = errors.New("default role not found")
	ErrDefaultRolesMany    = errors.New("multiple default roles found")
)
