package service

import "errors"

var (
	ErrAccountUserExists           = NewConflictError("user exists in the account")
	ErrChangeAccountStatusConflict = NewConflictError("cannot change account status due to conflict")
)

var (
	ErrChangeAccountStatusForbidden = NewForbiddenError("insufficient rights to perform this action")
)

var (
	ErrInvalidStatus = errors.New("invalid status")
)
