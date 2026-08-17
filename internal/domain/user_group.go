package domain

import (
	"time"
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
	// JoinedAt — время добавления участника в группу.
	JoinedAt time.Time
}

func (m *GroupMember) FromDB(member *schema.GroupMember) {
	m.UserID = member.UserID
	m.GroupID = member.GroupID
	m.RoleID = member.RoleID
	m.JoinedAt = member.JoinedAt
}

// GroupMemberDetails — карточка участника группы для списка участников (§3.2 дизайна эпика
// Э2, П-3): данные пользователя, его роль в группе и время вступления, собранные из членства,
// батча пользователей и ролей группы аккаунта.
type GroupMemberDetails struct {
	UserID         uuid.UUID
	Name           string
	Surname        string
	Email          string
	RoleID         uuid.UUID
	RoleName       string
	PermissionMask PermissionMask
	JoinedAt       time.Time
}
