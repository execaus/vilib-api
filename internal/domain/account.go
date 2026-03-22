package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"vilib-api/internal/gen/schema"
)

const (
	// AccountUserBitPosition Обычный пользователь системы.
	AccountUserBitPosition BitPosition = iota

	// AccountModeratorBitPosition Модератор аккаунта. Имеет следующие возможности:
	// - добавлять новых пользователей в аккаунт.
	AccountModeratorBitPosition

	// AccountAdminBitPosition Администратор аккаунта. Имеет следующие возможности:
	// - добавлять новых пользователей в аккаунт.
	AccountAdminBitPosition

	// AccountSuperAdminBitPosition Супер администратор аккаунта, является владельцем аккаунта.
	// Имеет следующие возможности:
	// Повышать до администратора.
	// Отдать роль супер администратора обычному администратору.
	AccountSuperAdminBitPosition
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

func NameFromEmail(email string) (string, error) {
	if i := strings.Index(email, "@"); i != -1 {
		return email[:i], nil
	}

	return "", errors.New(fmt.Sprintf("invalid email: %s", email))
}
