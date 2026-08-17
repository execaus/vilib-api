package dto

import "github.com/google/uuid"

type RegisterRequest struct {
	Name    string `json:"name"    binding:"required,min=2,max=64"`
	Surname string `json:"surname" binding:"required,min=2,max=64"`
	Email   string `json:"email"   binding:"required,email,max=64"`
}

type RegisterResponse struct{}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email,max=64"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// SwitchAccountRequest — тело запроса переключения текущей организации сессии (§2.4 дизайна
// эпика Э2).
type SwitchAccountRequest struct {
	AccountID uuid.UUID `json:"account_id" binding:"required"`
}

// SwitchAccountResponse — новый токен с изменённым current_account_id, тот же формат, что
// LoginResponse.
type SwitchAccountResponse struct {
	Token string `json:"token"`
}
