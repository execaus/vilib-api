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

// UpdateAccountRoleRequest — тело PUT accounts/{accountId}/roles/{roleId}: полная замена всех
// редактируемых полей роли аккаунта (§4 дизайна эпика Э2), в отличие от частичного
// UpdateUserRequest.
type UpdateAccountRoleRequest struct {
	Name       string                `json:"name"       binding:"required,max=64"`
	Permission domain.PermissionMask `json:"permission"`
	ParentID   *uuid.UUID            `json:"parent_id"`
	IsDefault  bool                  `json:"is_default"`
}

type UpdateAccountRoleResponse struct {
	AccountRole AccountRole `json:"account_role"`
}

type AccountRole struct {
	ID         uuid.UUID             `json:"id"`
	Name       string                `json:"name"`
	Permission domain.PermissionMask `json:"permission"`
	ParentID   *uuid.UUID            `json:"parent_id"`
	IsDefault  bool                  `json:"is_default"`
	// IsSystem — признак системной роли (например, владелец аккаунта), созданной автоматически
	// и недоступной для удаления/переименования (§3.4 дизайна эпика Э2).
	IsSystem bool `json:"is_system"`
}

func (r *AccountRole) FromDomain(role domain.AccountRole) {
	r.ID = role.ID
	r.Name = role.Name
	r.Permission = role.PermissionMask
	r.ParentID = role.ParentID
	r.IsDefault = role.IsDefault
	r.IsSystem = role.IsSystem
}
