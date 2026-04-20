package domain

import (
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

const (
	AccountOwnerSystemRoleName = "owner"
)

const (
	// AccountPermissionOwner владелец аккаунта.
	AccountPermissionOwner PermissionFlag = iota
	// AccountPermissionCreateUser разрешено ли создавать пользователей внутри организации.
	AccountPermissionCreateUser
	// AccountPermissionCreateAccountRole разрешено ли создавать роли аккаунта.
	AccountPermissionCreateAccountRole
	// AccountPermissionVideoWatch разрешено ли смотреть видео.
	AccountPermissionVideoWatch
	// AccountPermissionVideoUpload разрешено ли загружать видео.
	AccountPermissionVideoUpload
	// AccountPermissionVideoEdit разрешено ли редактировать видео.
	AccountPermissionVideoEdit
)

type AccountRole struct {
	ID             uuid.UUID
	Name           string
	AccountID      uuid.UUID
	PermissionMask PermissionMask
	IsSystem       bool
	IsDefault      bool
	ParentID       *uuid.UUID
}

func (r *AccountRole) FromDB(db *schema.AccountRole) {
	r.ID = db.AccountRoleID
	r.Name = db.Name
	r.AccountID = db.AccountID
	r.PermissionMask = db.PermissionMask
	r.ParentID = dbconv.NullValToPtr(db.ParentRoleID)
	r.IsDefault = db.IsDefault
	r.IsSystem = db.IsSystem
}
