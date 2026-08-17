package dto

// ErrorMessage — тело ответа об ошибке. Code — машинный код ошибки для фронта (см. §6.8 ТЗ),
// Message — человекочитаемое техническое описание (англ.).
type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
