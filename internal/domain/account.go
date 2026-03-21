package domain

import (
	"time"
	"vilib-api/internal/gen/schema"
)

type Account struct {
	ID        string
	Name      string
	Email     string
	CreatedAt time.Time
}

func (a *Account) FromDB(db *schema.Account) {
	a.ID = db.AccountID.String()
	a.Name = db.Name
	a.Email = db.Email
	a.CreatedAt = db.CreatedAt
}
