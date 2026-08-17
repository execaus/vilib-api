package service

import "errors"

var (
	ErrAccountUserExists           = NewConflictErrorCode("conflict.user_exists", "user exists in the account")
	ErrChangeAccountStatusConflict = NewConflictError("cannot change account status due to conflict")
	// ErrUserDeactivated — конфликт состояния при повторной деактивации (HTTP 409
	// conflict.user_deactivated). Также используется как sentinel деактивированной строки при
	// входе — в этом случае handler.sendServiceErrorWithDeactivated переопределяет код на
	// 401 user_deactivated (§2.2 дизайна эпика).
	ErrUserDeactivated   = NewConflictErrorCode("conflict.user_deactivated", "user is deactivated")
	ErrUserAlreadyActive = NewConflictErrorCode("conflict.user_active", "user is already active")
	ErrIsSystemRole      = NewConflictErrorCode("conflict.system_role", "cannot perform action on system role")
	ErrIsOwner           = NewConflictErrorCode("conflict.account_owner", "cannot deactivate account owner")
	ErrRoleInUse         = NewConflictErrorCode("conflict.role_in_use", "role is assigned to active users")
	ErrGroupRoleInUse    = NewConflictErrorCode("conflict.group_role_in_use", "role is assigned to group members")
)

var (
	ErrChangeAccountStatusForbidden = NewForbiddenError("insufficient rights to perform this action")
)

var (
	ErrInvalidStatus = errors.New("invalid status")
)
