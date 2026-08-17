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
	// ErrDefaultRoleRequired — попытка снять флаг is_default с роли (аккаунта или группы),
	// являющейся сейчас единственной ролью по умолчанию (HTTP 409 conflict.default_role_required):
	// аккаунт/группа не могут остаться без роли по умолчанию (§4 дизайна эпика Э2).
	ErrDefaultRoleRequired = NewConflictErrorCode(
		"conflict.default_role_required",
		"cannot unset default flag from the only default role",
	)
)

var (
	ErrChangeAccountStatusForbidden = NewForbiddenError("insufficient rights to perform this action")
	// ErrForbiddenUserDeactivated — деактивированная строка пользователя пытается получить
	// собственный профиль или переключить организацию (HTTP 403 forbidden.user_deactivated,
	// §2.3, §2.4 дизайна эпика Э2). В отличие от ErrUserDeactivated (409, конфликт состояния
	// при повторной деактивации/входе) — это отказ в доступе к действию, а не конфликт.
	ErrForbiddenUserDeactivated = NewForbiddenErrorCode("forbidden.user_deactivated", "user is deactivated")
)

var (
	ErrInvalidStatus = errors.New("invalid status")
)
