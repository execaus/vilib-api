package service

import "errors"

var (
	ErrAccountUserExists           = NewConflictError("user exists in the account")
	ErrChangeAccountStatusConflict = NewConflictError("cannot change account status due to conflict")
	ErrUserDeactivated             = NewConflictError("user is deactivated")
	ErrUserAlreadyActive           = NewConflictError("user is already active")
	ErrIsSystemRole                = NewConflictError("cannot perform action on system role")
	ErrIsOwner                     = NewConflictError("cannot deactivate account owner")
	ErrRoleInUse                   = NewConflictError("role is assigned to active users")
	ErrGroupRoleInUse              = NewConflictError("role is assigned to group members")
)

var (
	ErrChangeAccountStatusForbidden = NewForbiddenError("insufficient rights to perform this action")
)

var (
	ErrInvalidStatus = errors.New("invalid status")
)
