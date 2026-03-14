package models

import (
	"time"
	"vilib-api/internal/gen/schema"
)

type User struct {
	ID           string
	Name         string
	Surname      string
	PasswordHash string
	Email        string
	CreatedAt    time.Time
}

func (u User) FromDB(db *schema.User) {
	u.ID = db.UserID.String()
	u.Name = db.Name
	u.Surname = db.Surname
	u.PasswordHash = db.PasswordHash
	u.Email = db.Email
	u.CreatedAt = db.CreatedAt
}
