package domain

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AuthClaims struct {
	jwt.RegisteredClaims

	UserID           uuid.UUID   `json:"user_id"`
	CurrentAccountID uuid.UUID   `json:"current_account_id"`
	Accounts         []uuid.UUID `json:"accounts"`
}
