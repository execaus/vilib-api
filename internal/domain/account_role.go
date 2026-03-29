package domain

import (
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type AccountRole struct {
	ID             uuid.UUID
	Name           string
	PermissionMask PermissionMask
	IsSystem       bool
	IsDefault      bool
	ParentID       *uuid.UUID
}

func (r *AccountRole) FromDB(db *schema.AccountRole) {
	r.ID = db.AccountRoleID
	r.Name = db.Name
	r.PermissionMask = db.PermissionMask
	r.ParentID = dbconv.NullValToPtr(db.ParentRoleID)
	r.IsDefault = db.IsDefault
	r.IsSystem = db.IsSystem
}
