package domain

import (
	"time"
	"vilib-api/internal/gen/schema"
)

type Account struct {
	ID        string
	Name      string
	OwnerID   string
	Email     string
	CreatedAt time.Time
}

func (a *Account) FromDB(db *schema.Account) {
	a.ID = db.AccountID.String()
	a.Name = db.Name
	a.OwnerID = db.OwnerID.String()
	a.Email = db.Email
	a.CreatedAt = db.CreatedAt
}
