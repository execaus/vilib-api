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

// ChangePasswordRequest — тело POST auth/password/change (§6 дизайна эпика Э2, поправка О-1):
// меняет пароль только текущей строки пользователя (аккаунт из JWT).
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// ChangePasswordResponse — тело ответа 204, пустое (совпадает с телом ручки в спецификации).
type ChangePasswordResponse struct{}

// ForgotPasswordRequest — тело POST auth/password/forgot (§6 дизайна эпика Э2, поправка О-1).
// AccountID необязателен: не задан и активная организация одна — используется она, несколько —
// письмо со списком организаций и отдельной ссылкой на каждую.
type ForgotPasswordRequest struct {
	Email     string     `json:"email"      binding:"required,email,max=64"`
	AccountID *uuid.UUID `json:"account_id"`
}

// ForgotPasswordResponse — тело ответа 200, всегда пустое независимо от существования email
// (§6 дизайна эпика Э2).
type ForgotPasswordResponse struct{}

// ResetPasswordRequest — тело POST auth/password/reset (§6 дизайна эпика Э2, поправка О-1).
type ResetPasswordRequest struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// ResetPasswordResponse — тело ответа 200, пустое.
type ResetPasswordResponse struct{}
