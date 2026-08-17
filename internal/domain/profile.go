package domain

import "github.com/google/uuid"

// Profile — агрегированный контекст текущего пользователя для ручки GET /me (§2.3 дизайна
// эпика Э2): организация текущей строки, все организации по email с активной строкой, роль
// в текущей организации, признак владельца аккаунта и членства в группах.
type Profile struct {
	User     User
	Account  Account
	Accounts []Account
	Role     AccountRole
	IsOwner  bool
	Groups   []GroupMembership
}

// GroupMembership — членство пользователя в группе вместе с названием группы и данными его
// роли в ней.
type GroupMembership struct {
	GroupID        uuid.UUID
	GroupName      string
	RoleID         uuid.UUID
	RoleName       string
	PermissionMask PermissionMask
}
