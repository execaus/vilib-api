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

// UpdateGroupRoleRequest — тело PUT accounts/{accountId}/user-groups/roles/{roleId}: полная
// замена всех редактируемых полей роли группы (§4 дизайна эпика Э2). В отличие от роли
// аккаунта бит GroupPermissionOwner в PermissionMask разрешён.
type UpdateGroupRoleRequest struct {
	Name           string                `json:"name"            binding:"required,max=64"`
	PermissionMask domain.PermissionMask `json:"permission_mask"`
	IsDefault      bool                  `json:"is_default"`
}

type UpdateGroupRoleResponse struct {
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
