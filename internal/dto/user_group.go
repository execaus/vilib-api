package dto

import (
	"vilib-api/internal/domain"

	"github.com/google/uuid"
)

type CreateUserGroupRequest struct {
	Name  string      `json:"name"  binding:"required,max=64"`
	Users []uuid.UUID `json:"users" binding:"required"`
}

type CreateUserGroupResponse struct {
	ID    uuid.UUID     `json:"id"`
	Name  string        `json:"name"`
	Users []GroupMember `json:"users"`
}

type GroupMember struct {
	UserID  uuid.UUID `json:"user_id"`
	GroupID uuid.UUID `json:"group_id"`
	RoleID  uuid.UUID `json:"role_id"`
}

func (m GroupMember) FromDomain(member domain.GroupMember) {
	m.UserID = member.UserID
	m.GroupID = member.GroupID
	m.RoleID = member.RoleID
}

type AddGroupMemberRequest struct {
	Users []uuid.UUID `json:"users"`
}

type AddGroupMemberResponse struct {
	Members []GroupMember `json:"members"`
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
