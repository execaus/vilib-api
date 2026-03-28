package domain

import (
	"errors"
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
	if i := strings.Index(email, "@"); i != -1 {
		return email[:i], nil
	}

	return "", errors.New(fmt.Sprintf("invalid email: %s", email))
}
