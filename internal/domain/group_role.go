package domain

import (
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

const (
	// GroupPermissionOwner владелец группы (имеет все права).
	GroupPermissionOwner PermissionFlag = iota
	// GroupPermissionManageMembers разрешено ли управлять участниками группы.
	GroupPermissionManageMembers
	// GroupPermissionVideoWatch разрешено ли смотреть видео в группе.
	GroupPermissionVideoWatch
	// GroupPermissionManageVideo разрешено ли загружать, редактировать и удалять видео в группе.
	GroupPermissionManageVideo
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
