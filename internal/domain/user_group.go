package domain

import (
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type UserGroup struct {
	ID        uuid.UUID
	Name      string
	AccountID uuid.UUID
}

func (u *UserGroup) FromDB(db *schema.UserGroup) {
	u.ID = db.GroupID
	u.Name = db.Name
	u.AccountID = db.AccountID
}

type GroupMember struct {
	GroupID uuid.UUID
	UserID  uuid.UUID
	RoleID  uuid.UUID
}

func (m *GroupMember) FromDB(member *schema.GroupMember) {
	m.UserID = member.UserID
	m.GroupID = member.GroupID
	m.RoleID = member.RoleID
}
