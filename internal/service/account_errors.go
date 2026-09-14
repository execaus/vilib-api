package service

var (
	// ErrAccountNameInvalid — название организации при регистрации после обрезки пробелов вне
	// допустимой длины (HTTP 400 validation.account_name, A-01 ТЗ).
	ErrAccountNameInvalid = NewValidationErrorCode(
		"validation.account_name",
		"account name must be between 2 and 128 characters",
	)
)
