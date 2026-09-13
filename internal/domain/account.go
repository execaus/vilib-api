package domain

import (
	"fmt"
	"strings"
	"time"
	"vilib-api/internal/gen/schema"

	"github.com/google/uuid"
)

type Account struct {
	ID        uuid.UUID
	Name      string
	Email     string
	CreatedAt time.Time
}

func (a *Account) FromDB(db *schema.Account) {
	a.ID = db.AccountID
	a.Name = db.Name
	a.Email = db.Email
	a.CreatedAt = db.CreatedAt
}

func NameFromEmail(email string) (string, error) {
	if name, _, found := strings.Cut(email, "@"); found {
		return name, nil
	}

	return "", fmt.Errorf("invalid email: %s", email)
}
