package domain

import (
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"
)

type AccountRole struct {
	ID             string
	Name           string
	PermissionMask PermissionMask
	IsDefault      bool
	ParentID       *string
}

func (r *AccountRole) FromDB(db *schema.AccountRole) {
	r.ID = db.AccountRoleID.String()
	r.Name = db.Name
	r.PermissionMask = db.PermissionMask
	r.ParentID = dbconv.NullUUIDToStrPtr(db.ParentRoleID)
	r.IsDefault = db.IsDefault
}
