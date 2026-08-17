package dto

import (
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

// MeResponse — агрегированный контекст текущего пользователя (§2.3 дизайна эпика Э2).
type MeResponse struct {
	User     MeUser         `json:"user"`
	Account  AccountBrief   `json:"account"`
	Accounts []AccountBrief `json:"accounts"`
	Role     AccountRole    `json:"role"`
	IsOwner  bool           `json:"is_owner"`
	Groups   []MyGroup      `json:"groups"`
}

// FromDomain заполняет ответ /me данными агрегированного профиля пользователя.
func (r *MeResponse) FromDomain(profile domain.Profile) {
	r.User.FromDomain(profile.User)
	r.Account.FromDomain(profile.Account)

	r.Accounts = make([]AccountBrief, len(profile.Accounts))
	for i, account := range profile.Accounts {
		r.Accounts[i].FromDomain(account)
	}

	r.Role = AccountRole{}
	r.Role.FromDomain(profile.Role)

	r.IsOwner = profile.IsOwner

	r.Groups = make([]MyGroup, len(profile.Groups))
	for i, membership := range profile.Groups {
		r.Groups[i].FromDomain(membership)
	}
}

// MeUser — данные текущего пользователя в ответе /me.
type MeUser struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Surname string    `json:"surname"`
	Email   string    `json:"email"`
}

// FromDomain заполняет данные пользователя из domain.User.
func (u *MeUser) FromDomain(user domain.User) {
	u.ID = user.ID
	u.Name = user.Name
	u.Surname = user.Surname
	u.Email = user.Email
}

// AccountBrief — краткие данные организации (id, name) для списков в ответе /me.
type AccountBrief struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// FromDomain заполняет краткие данные организации из domain.Account.
func (a *AccountBrief) FromDomain(account domain.Account) {
	a.ID = account.ID
	a.Name = account.Name
}

// MyGroup — краткие данные группы и роли текущего пользователя в ней для ответа /me.
type MyGroup struct {
	ID             uuid.UUID             `json:"id"`
	Name           string                `json:"name"`
	RoleID         uuid.UUID             `json:"role_id"`
	RoleName       string                `json:"role_name"`
	PermissionMask domain.PermissionMask `json:"permission_mask"`
}

// FromDomain заполняет данные членства из domain.GroupMembership.
func (g *MyGroup) FromDomain(membership domain.GroupMembership) {
	g.ID = membership.GroupID
	g.Name = membership.GroupName
	g.RoleID = membership.RoleID
	g.RoleName = membership.RoleName
	g.PermissionMask = membership.PermissionMask
}
