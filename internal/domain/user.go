package domain

import (
	"time"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Name         string
	Surname      string
	PasswordHash string
	Email        string
	RoleID       uuid.UUID
	CreatedAt    time.Time
}

func (u *User) FromDB(db *schema.User) {
	u.ID = db.UserID
	u.Name = db.Name
	u.Surname = db.Surname
	u.PasswordHash = db.PasswordHash
	u.Email = db.Email
	u.RoleID = db.RoleID
	u.CreatedAt = db.CreatedAt
}
