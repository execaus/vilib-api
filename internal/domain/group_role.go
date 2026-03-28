package domain

import (
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type GroupRole struct {
	ID             uuid.UUID
	Name           string
	PermissionMask PermissionMask
	AccountID      uuid.UUID
	IsDefault      bool
}

func (r *GroupRole) FromDB(role *schema.GroupRole) {
	r.ID = role.GroupRoleID
	r.Name = role.Name
	r.PermissionMask = role.PermissionMask
	r.AccountID = role.AccountID
	r.IsDefault = role.IsDefault
}
