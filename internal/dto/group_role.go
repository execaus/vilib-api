package dto

import (
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type CreateGroupRoleRequest struct {
	Name           string                `json:"name"`
	PermissionMask domain.PermissionMask `json:"permission_mask"`
	IsDefault      bool                  `json:"is_default"`
}

type CreateGroupRoleResponse struct {
	Role GroupRole `json:"role"`
}

type GroupRole struct {
	ID             uuid.UUID             `json:"id"`
	Name           string                `json:"name"`
	PermissionMask domain.PermissionMask `json:"permission_mask"`
	AccountID      uuid.UUID             `json:"account_id"`
	IsDefault      bool                  `json:"is_default"`
}

func (r *GroupRole) FromDomain(role domain.GroupRole) {
	r.ID = role.ID
	r.Name = role.Name
	r.PermissionMask = role.PermissionMask
	r.AccountID = role.AccountID
	r.IsDefault = role.IsDefault
}
