package dto

import (
	"vilib-api/internal/domain"
)

type CreateAccountRoleRequest struct {
	Name       string                `json:"name"       binding:"required,min=2,max=32"`
	Permission domain.PermissionMask `json:"permission" binding:"required"`
	ParentID   *string               `json:"parent_id"  binding:"required"`
	IsDefault  bool                  `json:"is_default" binding:"required"`
}

type CreateAccountRoleResponse struct {
	AccountRoles []AccountRole `json:"account_roles"`
}

type AccountRole struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Permission domain.PermissionMask `json:"permission"`
	ParentID   *string               `json:"parent_id"`
	IsDefault  bool                  `json:"is_default"`
}

func (r *AccountRole) FromDomain(role domain.AccountRole) {
	r.ID = role.ID
	r.Name = role.Name
	r.Permission = role.PermissionMask
	r.ParentID = role.ParentID
	r.IsDefault = role.IsDefault
}
