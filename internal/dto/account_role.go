package dto

import (
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type CreateAccountRoleRequest struct {
	Name       string                `json:"name"`
	Permission domain.PermissionMask `json:"permission"`
	ParentID   *uuid.UUID            `json:"parent_id"`
	IsDefault  bool                  `json:"is_default"`
}

type CreateAccountRoleResponse struct {
	AccountRole AccountRole `json:"account_role"`
}

type AccountRole struct {
	ID         uuid.UUID             `json:"id"`
	Name       string                `json:"name"`
	Permission domain.PermissionMask `json:"permission"`
	ParentID   *uuid.UUID            `json:"parent_id"`
	IsDefault  bool                  `json:"is_default"`
}

func (r *AccountRole) FromDomain(role domain.AccountRole) {
	r.ID = role.ID
	r.Name = role.Name
	r.Permission = role.PermissionMask
	r.ParentID = role.ParentID
	r.IsDefault = role.IsDefault
}
