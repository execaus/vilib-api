package service

var (
	// ErrEmailInvalid — email не позволяет вычислить название организации (HTTP 400
	// validation.email, регистрация). Раньше был ConflictError (409) — исправлено §2.2
	// дизайна эпика.
	ErrEmailInvalid      = NewValidationErrorCode("validation.email", "invalid email")
	ErrAccountNameExists = NewConflictErrorCode("conflict.account_name", "account name exists")
)
