package domain

import (
	"github.com/golang-jwt/jwt/v5"
)

type AuthClaims struct {
	jwt.RegisteredClaims

	UserID           string   `json:"user_id"`
	CurrentAccountID string   `json:"current_account_id"`
	Accounts         []string `json:"accounts"`
}
