package dto

import (
	"time"
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type CreateUserGroupRequest struct {
	Name  string      `json:"name"  binding:"required,max=64"`
	Users []uuid.UUID `json:"users"`
}

type CreateUserGroupResponse struct {
	ID    uuid.UUID     `json:"id"`
	Name  string        `json:"name"`
	Users []GroupMember `json:"users"`
}

type GroupMember struct {
	UserID   uuid.UUID `json:"user_id"`
	GroupID  uuid.UUID `json:"group_id"`
	RoleID   uuid.UUID `json:"role_id"`
	JoinedAt time.Time `json:"joined_at"`
}

// FromDomain заполняет поля из доменной модели участника группы (pointer receiver — исправляет
// прежнюю реализацию со значимым получателем, которая не изменяла ничего, §3.3 дизайна эпика
// Э2, В-25).
func (m *GroupMember) FromDomain(member domain.GroupMember) {
	m.UserID = member.UserID
	m.GroupID = member.GroupID
	m.RoleID = member.RoleID
	m.JoinedAt = member.JoinedAt
}

type AddGroupMemberRequest struct {
	Users []uuid.UUID `json:"users"`
}

type AddGroupMemberResponse struct {
	Members []GroupMember `json:"members"`
}

// GroupMemberDetails — карточка участника группы для списка участников (§3.2 дизайна эпика
// Э2, П-3): данные пользователя, его роль в группе и время вступления.
type GroupMemberDetails struct {
	UserID         uuid.UUID             `json:"user_id"`
	Name           string                `json:"name"`
	Surname        string                `json:"surname"`
	Email          string                `json:"email"`
	RoleID         uuid.UUID             `json:"role_id"`
	RoleName       string                `json:"role_name"`
	PermissionMask domain.PermissionMask `json:"permission_mask"`
	JoinedAt       time.Time             `json:"joined_at"`
}

// FromDomain заполняет карточку участника из агрегированной доменной модели.
func (m *GroupMemberDetails) FromDomain(member domain.GroupMemberDetails) {
	m.UserID = member.UserID
	m.Name = member.Name
	m.Surname = member.Surname
	m.Email = member.Email
	m.RoleID = member.RoleID
	m.RoleName = member.RoleName
	m.PermissionMask = member.PermissionMask
	m.JoinedAt = member.JoinedAt
}

// GetGroupMembersResponse — тело ответа списка участников группы (§3.2 дизайна эпика Э2, П-3).
type GetGroupMembersResponse struct {
	Members []GroupMemberDetails `json:"members"`
}

// UpdateGroupMemberRequest — тело запроса смены роли участника группы (§3.3 дизайна эпика Э2,
// П-4).
type UpdateGroupMemberRequest struct {
	RoleID uuid.UUID `json:"role_id" binding:"required"`
}

// UpdateGroupMemberResponse — тело ответа смены роли участника группы (§3.3 дизайна эпика Э2,
// П-4).
type UpdateGroupMemberResponse struct {
	Member GroupMemberDetails `json:"member"`
}

type UserGroup struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	AccountID uuid.UUID `json:"account_id"`
}

func (g *UserGroup) FromDomain(group domain.UserGroup) {
	g.ID = group.ID
	g.Name = group.Name
	g.AccountID = group.AccountID
}

type GetAllUserGroupsResponse struct {
	Groups []UserGroup `json:"groups"`
}

// GetUserGroupResponse — тело ответа карточки группы (§3.2 дизайна эпика Э2, П-3).
type GetUserGroupResponse struct {
	Group UserGroup `json:"group"`
}

// UpdateUserGroupRequest — тело PUT accounts/{accountId}/user-groups/{groupId}: переименование
// группы (§4 дизайна эпика Э2, «Блок C — редактирование»).
type UpdateUserGroupRequest struct {
	Name string `json:"name" binding:"required,max=64"`
}

// UpdateUserGroupResponse — тело ответа переименования группы.
type UpdateUserGroupResponse struct {
	Group UserGroup `json:"group"`
}
