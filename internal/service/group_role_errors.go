package service

var (
	// ErrDefaultGroupRoleNotFound — у аккаунта нет роли группы по умолчанию (HTTP 409
	// conflict.default_group_role_missing, добавление участника в группу без явной роли).
	// Раньше был repository.ErrNotFound — попадал в HTTP 404 — исправлено §2.2 дизайна эпика.
	ErrDefaultGroupRoleNotFound = NewConflictErrorCode(
		"conflict.default_group_role_missing",
		"default group role not found",
	)
	// ErrGroupRoleNameExists — дубль имени роли группы в пределах аккаунта (HTTP 409
	// conflict.group_role_name).
	ErrGroupRoleNameExists = NewConflictErrorCode("conflict.group_role_name", "group role name already exists")
)
