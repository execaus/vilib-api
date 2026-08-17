package service

import "errors"

var (
	// ErrTokenInvalid — токен не проходит проверку подписи, формата или явно помечен
	// невалидным (token.Valid == false). HTTP 401 (§2.1 дизайна эпика).
	ErrTokenInvalid = NewUnauthorizedErrorCode("token_invalid", "invalid token")
	// ErrTokenExpired — срок действия токена истёк. HTTP 401 (§2.1 дизайна эпика).
	ErrTokenExpired = NewUnauthorizedErrorCode("token_expired", "token expired")
	// ErrInvalidCredentials — email/пароль не совпали ни в одной строке пользователя. HTTP 401
	// (§2.4 дизайна эпика).
	ErrInvalidCredentials = NewUnauthorizedErrorCode("invalid_credentials", "invalid credentials")
	// ErrNotAccountMember — у пользователя нет строки в организации, на которую он пытается
	// переключиться (HTTP 403 forbidden, §2.4 дизайна эпика Э2).
	ErrNotAccountMember = NewForbiddenError("user is not a member of the account")
	// ErrOldPasswordInvalid — старый пароль в ChangePassword не совпал с текущим хешем строки
	// (HTTP 400 validation.old_password, §6 дизайна эпика Э2).
	ErrOldPasswordInvalid = NewValidationErrorCode("validation.old_password", "old password is invalid")
	// ErrPasswordInvalid — новый пароль совпадает со старым или короче PasswordMinLength
	// (HTTP 400 validation.password, §6 дизайна эпика Э2).
	ErrPasswordInvalid = NewValidationErrorCode("validation.password", "new password is invalid")
	// ErrResetTokenInvalid — токен сброса пароля не найден, уже использован или просрочен
	// (HTTP 400 validation.reset_token, §6 дизайна эпика Э2).
	ErrResetTokenInvalid = NewValidationErrorCode("validation.reset_token", "reset token is invalid")
)

var (
	ErrAccountsNotFound = errors.New("accounts not found")
)
