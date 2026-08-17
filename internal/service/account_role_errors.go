package service

import "errors"

var (
	// ErrDefaultRoleNotFound — у аккаунта нет роли по умолчанию (HTTP 409
	// conflict.default_role_missing, создание пользователя без явной роли). Раньше была
	// обычной [errors.New] — попадала в HTTP 500 — исправлено §2.2 дизайна эпика.
	ErrDefaultRoleNotFound = NewConflictErrorCode("conflict.default_role_missing", "default role not found")
	// ErrAccountRoleNameExists — дубль имени роли аккаунта в пределах аккаунта (HTTP 409
	// conflict.account_role_name).
	ErrAccountRoleNameExists = NewConflictErrorCode("conflict.account_role_name", "account role name already exists")
)

var (
	ErrDefaultRolesMany = errors.New("multiple default roles found")
)
