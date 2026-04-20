package domain

import (
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

const (
	// GroupPermissionOwner владелец группы (имеет все права).
	GroupPermissionOwner PermissionFlag = iota
	// GroupPermissionAddMember разрешено ли добавлять участников в группу.
	GroupPermissionAddMember
	// GroupPermissionRemoveMember разрешено ли удалять участников из группы.
	GroupPermissionRemoveMember
	// GroupPermissionCreateVideo разрешено ли загружать видео в группу.
	GroupPermissionCreateVideo
	// GroupPermissionEditVideo разрешено ли редактировать видео в группе.
	GroupPermissionEditVideo
	// GroupPermissionDeleteVideo разрешено ли удалять видео из группы.
	GroupPermissionDeleteVideo
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
