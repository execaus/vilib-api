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
