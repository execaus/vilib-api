package dto

// HealthResponse — ответ проверки работоспособности сервиса.
type HealthResponse struct {
	// Status — состояние сервиса, при успешной проверке всегда "ok".
	Status string `json:"status"`
}
