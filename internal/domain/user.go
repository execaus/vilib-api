package domain

import (
	"time"
	"vilib-api/internal/dbconv"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID
	Name          string
	Surname       string
	PasswordHash  string
	Email         string
	RoleID        uuid.UUID
	CreatedAt     time.Time
	DeactivatedAt *time.Time
}

func (u *User) FromDB(db *schema.User) {
	u.ID = db.UserID
	u.Name = db.Name
	u.Surname = db.Surname
	u.PasswordHash = db.PasswordHash
	u.Email = db.Email
	u.RoleID = db.RoleID
	u.CreatedAt = db.CreatedAt
	u.DeactivatedAt = dbconv.NullValToPtr(db.DeactivatedAt)
}

// IsActive возвращает true, если пользователь активен.
func (u *User) IsActive() bool { return u.DeactivatedAt == nil }

// UserPatch — частичное обновление пользователя (§4 дизайна эпика Э2, «Блок C —
// редактирование»): nil-поле оставляет значение без изменений. Смена роли (RoleID) и правка
// чужого профиля требуют права ManageUsers; инициатор, правящий собственное ФИО без смены
// роли, — исключение без проверки прав.
type UserPatch struct {
	Name    *string
	Surname *string
	RoleID  *uuid.UUID
}
