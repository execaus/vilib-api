package domain

import (
	"vilib-api/internal/gen/schema"
)

type AccountStatus struct {
	AccountID string
	UserID    string
	Status    BitmapValue
}

func (s *AccountStatus) FromDB(account *schema.AccountStatus) {
	s.AccountID = account.AccountID.String()
	s.UserID = account.UserID.String()
	s.Status = account.Status
}
