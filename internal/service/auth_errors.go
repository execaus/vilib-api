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
)

var (
	ErrAccountsNotFound = errors.New("accounts not found")
)
